package tools

import (
	"fmt"
	"os"
	"strings"
)

func ApplyPatch(call ToolCall, workdir string) ToolResult {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return ToolResult{Tool: "apply_patch", Error: "path parameter required"}
	}

	searchBlock, ok := call.Arguments["search"].(string)
	if !ok {
		return ToolResult{Tool: "apply_patch", Error: "search parameter required"}
	}

	replaceBlock, ok := call.Arguments["replace"].(string)
	if !ok {
		return ToolResult{Tool: "apply_patch", Error: "replace parameter required"}
	}

	if searchBlock == "" {
		return ToolResult{Tool: "apply_patch", Error: "search block cannot be empty"}
	}

	fullPath, err := resolveRepositoryPath(workdir, path, false)
	if err != nil {
		return ToolResult{Tool: "apply_patch", Error: err.Error()}
	}
	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return ToolResult{Tool: "apply_patch", Error: err.Error()}
	}

	content := string(contentBytes)
	normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")
	normalizedSearch := strings.ReplaceAll(searchBlock, "\r\n", "\n")

	if strings.Contains(normalizedContent, normalizedSearch) {
		newContent := strings.Replace(normalizedContent, normalizedSearch, replaceBlock, 1)
		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return ToolResult{Tool: "apply_patch", Error: err.Error()}
		}
		return ToolResult{
			Tool:          "apply_patch",
			Result:        "Patch applied successfully",
			ModifiedFiles: []string{path},
		}
	}

	fuzzyMatch := findFuzzyMatch(normalizedContent, normalizedSearch)
	if fuzzyMatch != "" {
		newContent := strings.Replace(normalizedContent, fuzzyMatch, replaceBlock, 1)
		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return ToolResult{Tool: "apply_patch", Error: err.Error()}
		}
		return ToolResult{
			Tool:          "apply_patch",
			Result:        "Patch applied with fuzzy matching",
			ModifiedFiles: []string{path},
		}
	}

	return ToolResult{
		Tool:  "apply_patch",
		Error: fmt.Sprintf("Could not find search block in %s. Ensure it matches file content.", path),
	}
}

func findFuzzyMatch(content, search string) string {
	searchLines := strings.Split(strings.TrimSpace(search), "\n")
	contentLines := strings.Split(content, "\n")

	for i := 0; i <= len(contentLines)-len(searchLines); i++ {
		matched := true
		for j, searchLine := range searchLines {
			contentLine := contentLines[i+j]
			if strings.TrimSpace(searchLine) != strings.TrimSpace(contentLine) {
				matched = false
				break
			}
		}
		if matched {
			return strings.Join(contentLines[i:i+len(searchLines)], "\n")
		}
	}
	return ""
}
