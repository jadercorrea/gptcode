package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var defaultIgnoreDirs = map[string]bool{
	"node_modules":  true,
	"vendor":        true,
	"target":        true,
	"dist":          true,
	"build":         true,
	".git":          true,
	".svn":          true,
	".hg":           true,
	"__pycache__":   true,
	".pytest_cache": true,
	".venv":         true,
	"venv":          true,
	".tox":          true,
	"coverage":      true,
	".idea":         true,
	".vscode":       true,
	"tmp":           true,
	"temp":          true,
}

func ProjectMap(call ToolCall, workdir string) ToolResult {
	maxDepth := 3 // Default depth
	if d, ok := call.Arguments["max_depth"].(float64); ok {
		maxDepth = int(d)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Project Map (max_depth=%d):\n", maxDepth)

	err := filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(workdir, path)
		if relPath == "." {
			return nil
		}

		baseName := filepath.Base(path)

		if info.IsDir() && defaultIgnoreDirs[baseName] {
			return filepath.SkipDir
		}

		if strings.HasPrefix(baseName, ".") && baseName != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate depth
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth >= maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			fmt.Fprintf(&b, "%s📂 %s/\n", indent, filepath.Base(path))
		} else {
			fmt.Fprintf(&b, "%s📄 %s\n", indent, filepath.Base(path))
		}

		return nil
	})

	if err != nil {
		return ToolResult{Tool: "project_map", Error: err.Error()}
	}

	return ToolResult{
		Tool:   "project_map",
		Result: b.String(),
	}
}
