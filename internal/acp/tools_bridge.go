package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/tools"
)

// ToolsBridge adapts the internal tool executor to work with ACP.
// When the editor supports file system or terminal capabilities,
// it delegates those operations to the editor via ACP requests.
// Otherwise, it falls back to local execution.
type ToolsBridge struct {
	server  *Server
	workdir string
}

// NewToolsBridge creates a new ACP-aware tool executor.
func NewToolsBridge(server *Server, workdir string) *ToolsBridge {
	return &ToolsBridge{
		server:  server,
		workdir: workdir,
	}
}

// ExecuteTool executes a tool call, delegating to the editor when possible.
func (b *ToolsBridge) ExecuteTool(call tools.LLMToolCall, emitter UpdateEmitter) tools.ToolResult {
	caps := b.server.ClientCapabilitiesFor()

	// Emit tool call start
	emitter.EmitToolCallStart(call.ID, call.Name, call.Arguments)

	var result tools.ToolResult

	switch call.Name {
	case "read_file":
		if caps.FS != nil && caps.FS.ReadTextFile {
			result = b.readFileViaEditor(call)
		} else {
			result = tools.ExecuteToolFromLLM(call, b.workdir)
		}

	case "write_file":
		if caps.FS != nil && caps.FS.WriteTextFile {
			result = b.writeFileViaEditor(call)
		} else {
			result = tools.ExecuteToolFromLLM(call, b.workdir)
		}

	case "apply_patch":
		// apply_patch requires read+write: read the file, apply the patch, write back
		if caps.FS != nil && caps.FS.ReadTextFile && caps.FS.WriteTextFile {
			result = b.applyPatchViaEditor(call)
		} else {
			result = tools.ExecuteToolFromLLM(call, b.workdir)
		}

	case "run_command":
		if caps.Terminal {
			result = b.runCommandViaTerminal(call)
		} else {
			result = tools.ExecuteToolFromLLM(call, b.workdir)
		}

	default:
		// All other tools (search_code, list_files, project_map, etc.) run locally
		result = tools.ExecuteToolFromLLM(call, b.workdir)
	}

	// Emit tool call completion
	if result.Error != "" {
		emitter.EmitToolCallError(call.ID, result.Error)
	} else {
		emitter.EmitToolCallComplete(call.ID, truncate(result.Result, 500))
	}

	return result
}

// readFileViaEditor delegates file reading to the editor via ACP fs/read_text_file.
func (b *ToolsBridge) readFileViaEditor(call tools.LLMToolCall) tools.ToolResult {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return tools.ToolResult{Tool: "read_file", Error: fmt.Sprintf("parse args: %v", err)}
	}

	path, _ := args["path"].(string)
	if path == "" {
		return tools.ToolResult{Tool: "read_file", Error: "path parameter required"}
	}

	absPath := toAbsPath(b.workdir, path)

	resp, err := b.server.SendRequest(MethodFSReadTextFile, FSReadTextFileParams{
		Path: absPath,
	})
	if err != nil {
		// Fallback to local read on communication error
		b.server.log("fs/read_text_file failed, falling back to local: %v", err)
		return tools.ExecuteToolFromLLM(call, b.workdir)
	}
	if resp.Error != nil {
		return tools.ToolResult{Tool: "read_file", Error: resp.Error.Message}
	}

	// Parse the result
	resultBytes, _ := json.Marshal(resp.Result)
	var fsResult FSReadTextFileResult
	if err := json.Unmarshal(resultBytes, &fsResult); err != nil {
		return tools.ToolResult{Tool: "read_file", Error: fmt.Sprintf("parse response: %v", err)}
	}

	return tools.ToolResult{
		Tool:   "read_file",
		Result: fsResult.Content,
	}
}

// writeFileViaEditor delegates file writing to the editor.
func (b *ToolsBridge) writeFileViaEditor(call tools.LLMToolCall) tools.ToolResult {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return tools.ToolResult{Tool: "write_file", Error: fmt.Sprintf("parse args: %v", err)}
	}

	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return tools.ToolResult{Tool: "write_file", Error: "path parameter required"}
	}

	absPath := toAbsPath(b.workdir, path)

	// Request permission first
	permResp, err := b.server.SendRequest(MethodRequestPermission, RequestPermissionParams{
		Permissions: []Permission{
			{
				Type:        "fileWrite",
				Description: fmt.Sprintf("Write to %s", path),
				FilePath:    absPath,
			},
		},
	})
	if err != nil {
		b.server.log("Permission request failed, falling back to local: %v", err)
		return tools.ExecuteToolFromLLM(call, b.workdir)
	}

	// Check if permission was granted
	permBytes, _ := json.Marshal(permResp.Result)
	var permResult RequestPermissionResult
	json.Unmarshal(permBytes, &permResult)
	if !permResult.Granted {
		return tools.ToolResult{Tool: "write_file", Error: "Permission denied by user"}
	}

	// Write via editor
	resp, err := b.server.SendRequest(MethodFSWriteTextFile, FSWriteTextFileParams{
		Path:    absPath,
		Content: content,
	})
	if err != nil {
		b.server.log("fs/write_text_file failed, falling back to local: %v", err)
		return tools.ExecuteToolFromLLM(call, b.workdir)
	}
	if resp.Error != nil {
		return tools.ToolResult{Tool: "write_file", Error: resp.Error.Message}
	}

	return tools.ToolResult{
		Tool:          "write_file",
		Result:        fmt.Sprintf("File written successfully: %s (%d bytes)", path, len(content)),
		ModifiedFiles: []string{path},
	}
}

// applyPatchViaEditor reads from editor, applies patch locally, writes back via editor.
func (b *ToolsBridge) applyPatchViaEditor(call tools.LLMToolCall) tools.ToolResult {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return tools.ToolResult{Tool: "apply_patch", Error: fmt.Sprintf("parse args: %v", err)}
	}

	path, _ := args["path"].(string)
	search, _ := args["search"].(string)
	replace, _ := args["replace"].(string)

	if path == "" || search == "" {
		return tools.ToolResult{Tool: "apply_patch", Error: "path and search parameters required"}
	}

	absPath := toAbsPath(b.workdir, path)

	// Read current content from editor
	readResp, err := b.server.SendRequest(MethodFSReadTextFile, FSReadTextFileParams{Path: absPath})
	if err != nil {
		return tools.ExecuteToolFromLLM(call, b.workdir)
	}
	if readResp.Error != nil {
		return tools.ToolResult{Tool: "apply_patch", Error: readResp.Error.Message}
	}

	readBytes, _ := json.Marshal(readResp.Result)
	var readResult FSReadTextFileResult
	json.Unmarshal(readBytes, &readResult)

	// Apply patch
	if !strings.Contains(readResult.Content, search) {
		return tools.ToolResult{Tool: "apply_patch", Error: "Search pattern not found in file"}
	}

	newContent := strings.Replace(readResult.Content, search, replace, 1)

	// Write back via editor
	writeResp, err := b.server.SendRequest(MethodFSWriteTextFile, FSWriteTextFileParams{
		Path:    absPath,
		Content: newContent,
	})
	if err != nil {
		return tools.ToolResult{Tool: "apply_patch", Error: fmt.Sprintf("write failed: %v", err)}
	}
	if writeResp.Error != nil {
		return tools.ToolResult{Tool: "apply_patch", Error: writeResp.Error.Message}
	}

	return tools.ToolResult{
		Tool:          "apply_patch",
		Result:        fmt.Sprintf("Patch applied to %s", path),
		ModifiedFiles: []string{path},
	}
}

// runCommandViaTerminal delegates command execution to the editor's terminal.
func (b *ToolsBridge) runCommandViaTerminal(call tools.LLMToolCall) tools.ToolResult {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return tools.ToolResult{Tool: "run_command", Error: fmt.Sprintf("parse args: %v", err)}
	}

	command, _ := args["command"].(string)
	if command == "" {
		return tools.ToolResult{Tool: "run_command", Error: "command parameter required"}
	}

	// Block sudo
	if strings.Contains(command, "sudo ") || strings.HasPrefix(command, "sudo") {
		return tools.ToolResult{
			Tool:  "run_command",
			Error: "sudo commands not allowed in autonomous mode",
		}
	}

	// Request permission for command execution
	permResp, err := b.server.SendRequest(MethodRequestPermission, RequestPermissionParams{
		Permissions: []Permission{
			{
				Type:        "command",
				Description: fmt.Sprintf("Execute: %s", truncate(command, 80)),
				Command:     command,
			},
		},
	})
	if err != nil {
		// Fallback to local execution
		b.server.log("Permission request failed, falling back to local: %v", err)
		return runCommandLocal(command, b.workdir)
	}

	permBytes, _ := json.Marshal(permResp.Result)
	var permResult RequestPermissionResult
	json.Unmarshal(permBytes, &permResult)
	if !permResult.Granted {
		return tools.ToolResult{Tool: "run_command", Error: "Permission denied by user"}
	}

	// Create terminal via editor
	createResp, err := b.server.SendRequest(MethodTerminalCreate, TerminalCreateParams{
		Command: command,
		Cwd:     b.workdir,
	})
	if err != nil {
		b.server.log("terminal/create failed, falling back to local: %v", err)
		return runCommandLocal(command, b.workdir)
	}
	if createResp.Error != nil {
		return tools.ToolResult{Tool: "run_command", Error: createResp.Error.Message}
	}

	createBytes, _ := json.Marshal(createResp.Result)
	var createResult TerminalCreateResult
	json.Unmarshal(createBytes, &createResult)

	// Wait for exit
	exitResp, err := b.server.SendRequest(MethodTerminalWaitExit, map[string]string{
		"terminalId": createResult.TerminalID,
	})
	if err != nil {
		return tools.ToolResult{Tool: "run_command", Error: fmt.Sprintf("wait failed: %v", err)}
	}

	exitBytes, _ := json.Marshal(exitResp.Result)
	var exitResult TerminalWaitExitResult
	json.Unmarshal(exitBytes, &exitResult)

	// Get output
	outputResp, err := b.server.SendRequest(MethodTerminalOutput, map[string]string{
		"terminalId": createResult.TerminalID,
	})

	var output string
	if err == nil && outputResp.Result != nil {
		outBytes, _ := json.Marshal(outputResp.Result)
		var outResult struct {
			Output string `json:"output"`
		}
		json.Unmarshal(outBytes, &outResult)
		output = outResult.Output
	}

	// Release terminal
	b.server.SendRequest(MethodTerminalRelease, map[string]string{
		"terminalId": createResult.TerminalID,
	})

	result := tools.ToolResult{
		Tool:   "run_command",
		Result: output,
	}
	if exitResult.ExitCode != 0 {
		result.Error = fmt.Sprintf("exit code %d", exitResult.ExitCode)
	}

	return result
}

// runCommandLocal executes a command locally (fallback).
func runCommandLocal(command, workdir string) tools.ToolResult {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()

	result := tools.ToolResult{
		Tool:   "run_command",
		Result: string(output),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// toAbsPath converts a relative path to absolute based on the working directory.
func toAbsPath(workdir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs := filepath.Join(workdir, path)
	// Clean path to remove any ../ traversals
	clean, err := filepath.Abs(abs)
	if err != nil {
		return abs
	}
	return clean
}

// EnsureWorkdir resolves the working directory, using the session's if available.
func (b *ToolsBridge) EnsureWorkdir(sessionID string) string {
	session := b.server.GetSession(sessionID)
	if session != nil && session.WorkingDirectory != "" {
		return session.WorkingDirectory
	}
	if b.workdir != "" {
		return b.workdir
	}
	cwd, _ := os.Getwd()
	return cwd
}
