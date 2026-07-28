package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/observability"
	"github.com/jadercorrea/gptcode/internal/processutil"
	"gopkg.in/yaml.v3"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Tool          string   `json:"tool"`
	Result        string   `json:"result"`
	Error         string   `json:"error,omitempty"`
	Err           error    `json:"-"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	TokensSaved   int      `json:"tokens_saved,omitempty"`
}

func GetAvailableTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_file",
				"description": "Read the contents of a file in the current repository",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path to the file from repository root",
						},
						"start_line": map[string]interface{}{
							"type":        "integer",
							"description": "Optional 1-based first line to read",
						},
						"end_line": map[string]interface{}{
							"type":        "integer",
							"description": "Optional 1-based last line to read (inclusive)",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "list_files",
				"description": "List files in a directory of the current repository",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path to directory from repository root (empty for root)",
						},
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Optional glob pattern to filter files (e.g., '*.go', 'test_*.ex')",
						},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "run_command",
				"description": "Execute a shell command in the repository directory (use for tests, linting, etc.)",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Shell command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "search_code",
				"description": "Search for a pattern in code files using grep",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Search pattern (regex)",
						},
						"file_pattern": map[string]interface{}{
							"type":        "string",
							"description": "Optional file pattern to limit search (e.g., '*.go')",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_guideline",
				"description": "Read detailed coding guidelines from ~/.gptcode/guidelines/ directory. Use when you need language-specific guidance, naming conventions, or TDD workflow details.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"guideline": map[string]interface{}{
							"type":        "string",
							"description": "Guideline name: 'tdd', 'naming', or 'languages'",
							"enum":        []string{"tdd", "naming", "languages"},
						},
					},
					"required": []string{"guideline"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "write_file",
				"description": "Write content to a file (creates or overwrites). Use this to save edited files.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path to the file from repository root",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Complete file content to write",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "project_map",
				"description": "Get a tree-like view of the project structure",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"max_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum depth to traverse (default 3)",
						},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "apply_patch",
				"description": "Replace a block of text in a file",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path",
						},
						"search": map[string]interface{}{
							"type":        "string",
							"description": "Exact text block to replace",
						},
						"replace": map[string]interface{}{
							"type":        "string",
							"description": "New text block",
						},
					},
					"required": []string{"path", "search", "replace"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "find_relevant_files",
				"description": "Find files most relevant to a task by searching for keywords. Use this FIRST before browsing directories. Returns ranked list of files containing task-related keywords.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Keywords or description of what you're looking for (e.g., 'URL formatting scanner output')",
						},
						"file_types": map[string]interface{}{
							"type":        "string",
							"description": "Optional: comma-separated extensions to filter (e.g., 'go,ts,py')",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum files to return (default: 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "web_search",
				"description": "Search the web for information, documentation, or answers. Use when you need to look up error messages, API docs, or general knowledge not in the codebase.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
						"num_results": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results (default: 5)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}

func ExecuteTool(call ToolCall, workdir string) ToolResult {
	return ExecuteToolContext(context.Background(), call, workdir)
}

func ExecuteToolContext(ctx context.Context, call ToolCall, workdir string) ToolResult {
	switch call.Name {
	case "read_file":
		return readFile(call, workdir)
	case "list_files":
		return listFiles(call, workdir)
	case "run_command":
		return runCommandContext(ctx, call, workdir)
	case "search_code":
		return searchCode(call, workdir)
	case "read_guideline":
		return readGuideline(call)
	case "write_file":
		return writeFile(call, workdir)
	case "project_map":
		return ProjectMap(call, workdir)
	case "apply_patch":
		return ApplyPatch(call, workdir)
	case "find_relevant_files":
		return FindRelevantFiles(call, workdir)
	case "web_search":
		return WebSearch(call)
	default:
		return ToolResult{
			Tool:  call.Name,
			Error: "Unknown tool",
		}
	}
}

type LLMToolCall struct {
	ID        string
	Name      string
	Arguments string
}

func ExecuteToolFromLLM(call LLMToolCall, workdir string) ToolResult {
	return ExecuteToolFromLLMContext(context.Background(), call, workdir)
}

func ExecuteToolFromLLMContext(ctx context.Context, call LLMToolCall, workdir string) ToolResult {
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &argsMap); err != nil {
		return ToolResult{
			Tool:  call.Name,
			Error: fmt.Sprintf("Failed to parse arguments: %v", err),
		}
	}

	toolCall := ToolCall{
		Name:      call.Name,
		Arguments: argsMap,
	}

	return ExecuteToolContext(ctx, toolCall, workdir)
}

func readFile(call ToolCall, workdir string) ToolResult {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return ToolResult{Tool: "read_file", Error: "path parameter required"}
	}

	fullPath, err := resolveRepositoryPath(workdir, path, false)
	if err != nil {
		return ToolResult{Tool: "read_file", Error: err.Error()}
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ToolResult{Tool: "read_file", Error: err.Error()}
	}

	result := string(content)
	lines := strings.Split(result, "\n")
	startLine, hasStart := numericArgument(call.Arguments, "start_line")
	endLine, hasEnd := numericArgument(call.Arguments, "end_line")
	if hasStart || hasEnd {
		if !hasStart {
			startLine = 1
		}
		if !hasEnd {
			endLine = startLine + 199
		}
		if startLine < 1 || endLine < startLine || startLine > len(lines) {
			return ToolResult{Tool: "read_file", Error: fmt.Sprintf(
				"invalid line range %d-%d for file with %d lines", startLine, endLine, len(lines))}
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}

		numbered := make([]string, 0, endLine-startLine+1)
		for lineNumber := startLine; lineNumber <= endLine; lineNumber++ {
			numbered = append(numbered, fmt.Sprintf("%d: %s", lineNumber, lines[lineNumber-1]))
		}
		return ToolResult{Tool: "read_file", Result: strings.Join(numbered, "\n")}
	}

	if len(lines) > 200 {
		truncated := strings.Join(lines[:200], "\n")
		result = truncated + fmt.Sprintf(
			"\n... (truncated, %d total lines; use start_line and end_line to read a specific range)",
			len(lines))
	}

	return ToolResult{
		Tool:   "read_file",
		Result: result,
	}
}

func numericArgument(arguments map[string]interface{}, name string) (int, bool) {
	value, ok := arguments[name]
	if !ok {
		return 0, false
	}
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	default:
		return 0, false
	}
}

func listFiles(call ToolCall, workdir string) ToolResult {
	pathArg, _ := call.Arguments["path"].(string)
	pattern, _ := call.Arguments["pattern"].(string)

	targetPath, err := resolveRepositoryPath(workdir, pathArg, false)
	if err != nil {
		return ToolResult{Tool: "list_files", Error: err.Error()}
	}

	var files []string
	err = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(workdir, path)

		if pattern != "" {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return ToolResult{Tool: "list_files", Error: err.Error()}
	}

	result := strings.Join(files, "\n")
	if len(files) > 30 {
		truncated := strings.Join(files[:30], "\n")
		result = truncated + fmt.Sprintf("\n... (%d more files, %d total)", len(files)-30, len(files))
	}

	return ToolResult{
		Tool:   "list_files",
		Result: result,
	}
}

func runCommandContext(ctx context.Context, call ToolCall, workdir string) ToolResult {
	command, ok := call.Arguments["command"].(string)
	if !ok {
		return ToolResult{Tool: "run_command", Error: "command parameter required"}
	}

	// Block sudo commands to prevent password prompts
	if strings.Contains(command, "sudo ") || strings.HasPrefix(command, "sudo") {
		return ToolResult{
			Tool:  "run_command",
			Error: "sudo commands not allowed in autonomous mode. Present sudo commands to user for manual execution.",
		}
	}

	output, err := processutil.CombinedOutput(ctx, workdir, "sh", "-c", command)

	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	fe := NewFilterEngine()
	filterRes := fe.Filter(command, string(output), exitCode)

	result := ToolResult{
		Tool:        "run_command",
		Result:      filterRes.FilteredOutput,
		TokensSaved: filterRes.TokensSaved,
	}

	if err != nil {
		result.Error = err.Error()
		result.Err = err
	}

	return result
}

func searchCode(call ToolCall, workdir string) ToolResult {
	pattern, ok := call.Arguments["pattern"].(string)
	if !ok {
		return ToolResult{Tool: "search_code", Error: "pattern parameter required"}
	}

	filePattern, _ := call.Arguments["file_pattern"].(string)

	args := []string{"-r", "-n", pattern}
	if filePattern != "" {
		args = append(args, "--include="+filePattern)
	}
	args = append(args, workdir)

	cmd := exec.Command("grep", args...)
	output, err := cmd.CombinedOutput()

	result := ToolResult{
		Tool:   "search_code",
		Result: string(output),
	}

	if err != nil && len(output) == 0 {
		result.Error = "No matches found"
	}

	return result
}

func readGuideline(call ToolCall) ToolResult {
	guideline, ok := call.Arguments["guideline"].(string)
	if !ok {
		return ToolResult{Tool: "read_guideline", Error: "guideline parameter required"}
	}

	validGuidelines := map[string]bool{
		"tdd":       true,
		"naming":    true,
		"languages": true,
	}

	if !validGuidelines[guideline] {
		return ToolResult{
			Tool:  "read_guideline",
			Error: fmt.Sprintf("invalid guideline '%s'. Must be one of: tdd, naming, languages", guideline),
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ToolResult{Tool: "read_guideline", Error: fmt.Sprintf("could not get home dir: %v", err)}
	}

	guidelinePath := filepath.Join(home, ".gptcode", "guidelines", guideline+".md")
	content, err := os.ReadFile(guidelinePath)
	if err != nil {
		return ToolResult{Tool: "read_guideline", Error: fmt.Sprintf("could not read guideline: %v", err)}
	}

	return ToolResult{
		Tool:   "read_guideline",
		Result: string(content),
	}
}

func writeFile(call ToolCall, workdir string) ToolResult {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return ToolResult{Tool: "write_file", Error: "path parameter required"}
	}

	content, ok := call.Arguments["content"].(string)
	if !ok {
		return ToolResult{Tool: "write_file", Error: "content parameter required"}
	}

	fullPath, err := resolveRepositoryPath(workdir, path, true)
	if err != nil {
		return ToolResult{Tool: "write_file", Error: err.Error()}
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResult{Tool: "write_file", Error: fmt.Sprintf("could not create directory: %v", err)}
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ToolResult{Tool: "write_file", Error: err.Error()}
	}

	return ToolResult{
		Tool:          "write_file",
		Result:        fmt.Sprintf("File written successfully: %s (%d bytes)", path, len(content)),
		ModifiedFiles: []string{path},
	}
}

// ExecuteToolWithObserver wraps ExecuteToolFromLLM and emits events to the observer
func ExecuteToolWithObserver(call LLMToolCall, workdir string, observer observability.Observer) ToolResult {
	return ExecuteToolWithObserverContext(context.Background(), call, workdir, observer)
}

func ExecuteToolWithObserverContext(ctx context.Context, call LLMToolCall, workdir string, observer observability.Observer) ToolResult {
	start := time.Now()
	existedBefore := make(map[string]bool)
	var arguments map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &arguments); err == nil {
		if path, ok := arguments["path"].(string); ok {
			_, err := os.Stat(filepath.Join(workdir, path))
			existedBefore[path] = err == nil
		}
	}

	// Execute the tool
	result := ExecuteToolFromLLMContext(ctx, call, workdir)

	// Emit events if observer is provided
	if observer != nil {
		// Emit tool call event
		truncatedResult := result.Result
		if len(truncatedResult) > 200 {
			truncatedResult = truncatedResult[:200] + "..."
		}

		observer.Emit(&observability.ToolCallEvent{
			BaseEvent:   observability.BaseEvent{Time: time.Now()},
			Name:        call.Name,
			Arguments:   call.Arguments,
			Result:      truncatedResult,
			Duration:    time.Since(start),
			Error:       result.Error,
			TokensSaved: result.TokensSaved,
		})

		// Emit file modification events
		for _, file := range result.ModifiedFiles {
			// Determine operation type (create vs modify)
			operation := "create"
			fullPath := filepath.Join(workdir, file)
			if existedBefore[file] {
				operation = "modify"
			}

			var bytes int64
			if info, err := os.Stat(fullPath); err == nil {
				bytes = info.Size()
			}

			observer.Emit(&observability.FileModifiedEvent{
				BaseEvent: observability.BaseEvent{Time: time.Now()},
				Path:      file,
				Operation: operation,
				Bytes:     bytes,
			})
		}
	}

	return result
}

func WebSearch(call ToolCall) ToolResult {
	query, ok := call.Arguments["query"].(string)
	if !ok || query == "" {
		return ToolResult{Tool: "web_search", Error: "query parameter required"}
	}

	numResults := 5
	if n, ok := call.Arguments["num_results"].(float64); ok {
		numResults = int(n)
	}

	results, err := searchWeb(query, numResults)
	if err != nil {
		return ToolResult{Tool: "web_search", Error: err.Error()}
	}

	return ToolResult{
		Tool:   "web_search",
		Result: results,
	}
}

func searchWeb(query string, numResults int) (string, error) {
	// Check for Tavily first (more popular for AI agents)
	// Check env var first, then keys.yaml
	tavilyKey := os.Getenv("TAVILY_API_KEY")
	if tavilyKey == "" {
		tavilyKey = loadSearchKey("TAVILY_API_KEY")
	}
	if tavilyKey != "" {
		return searchTavily(query, numResults, tavilyKey)
	}

	// Check for Exa
	exaKey := os.Getenv("EXA_API_KEY")
	if exaKey == "" {
		exaKey = loadSearchKey("EXA_API_KEY")
	}
	if exaKey != "" {
		return searchExa(query, numResults, exaKey)
	}

	// Check generic search API key
	searchKey := os.Getenv("SEARCH_API_KEY")
	if searchKey == "" {
		searchKey = loadSearchKey("SEARCH_API_KEY")
	}
	if searchKey != "" {
		// Try Exa first, fall back to Tavily
		if result, err := searchExa(query, numResults, searchKey); err == nil {
			return result, nil
		}
		return searchTavily(query, numResults, searchKey)
	}

	return searchFallback(query, numResults)
}

func loadSearchKey(envVar string) string {
	// Try to load from config package
	if val, err := getConfigKey(envVar); err == nil && val != "" {
		return val
	}
	return ""
}

func getConfigKey(key string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	keysPath := filepath.Join(home, ".gptcode", "keys.yaml")
	data, err := os.ReadFile(keysPath)
	if err != nil {
		return "", err
	}

	var keys map[string]string
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return "", err
	}

	// Try exact match
	if val, ok := keys[key]; ok {
		return val, nil
	}

	// Try without _API_KEY suffix
	cleanKey := strings.TrimSuffix(key, "_API_KEY")
	if val, ok := keys[cleanKey]; ok {
		return val, nil
	}

	return "", fmt.Errorf("key not found")
}

func searchExa(query string, numResults int, apiKey string) (string, error) {
	req, err := http.NewRequest("POST", "https://api.exa.ai/search", strings.NewReader(
		fmt.Sprintf(`{"query": %q, "num_results": %d}`, query, numResults)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Results) == 0 {
		return "No results found", nil
	}

	var output strings.Builder
	output.WriteString("Search Results:\n\n")
	for i, r := range result.Results {
		fmt.Fprintf(&output, "%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&output, "   URL: %s\n", r.URL)
		truncated := r.Content
		if len(truncated) > 300 {
			truncated = truncated[:300] + "..."
		}
		fmt.Fprintf(&output, "   %s\n\n", truncated)
	}

	return output.String(), nil
}

func searchTavily(query string, numResults int, apiKey string) (string, error) {
	reqBody := fmt.Sprintf(`{"query": %q, "max_results": %d}`, query, numResults)
	req, err := http.NewRequest("POST", "https://api.tavily.com/search", strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tavily API returned status %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Results) == 0 {
		return "No results found", nil
	}

	var output strings.Builder
	output.WriteString("Search Results:\n\n")
	for i, r := range result.Results {
		fmt.Fprintf(&output, "%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&output, "   URL: %s\n", r.URL)
		truncated := r.Content
		if len(truncated) > 300 {
			truncated = truncated[:300] + "..."
		}
		fmt.Fprintf(&output, "   %s\n\n", truncated)
	}

	return output.String(), nil
}

func searchFallback(query string, numResults int) (string, error) {
	return fmt.Sprintf(`Web search not configured. To enable, get a free API key:

# Option 1: Tavily (recommended - $1/month for 1k searches)
https://tavily.com
export TAVILY_API_KEY="your-key"

# Option 2: Exa (1k free/month)
https://exa.ai
export EXA_API_KEY="your-key"

# Then retry your search.
- Query: %s
- Search manually: https://www.google.com/search?q=%s`, query, query), nil
}
