package maestro

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func buildEditorRepositoryContext(root, language string, maxBytes int) (string, error) {
	extensions := map[string]bool{
		"go":         true,
		"elixir":     true,
		"rust":       true,
		"python":     true,
		"ruby":       true,
		"javascript": true,
		"typescript": true,
	}
	if !extensions[strings.ToLower(language)] || maxBytes <= 0 {
		return "", nil
	}

	var context strings.Builder
	used := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor", "node_modules":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !isEditorSourceFile(path, language) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if used+len(content) > maxBytes {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&context, "\n<file path=%q>\n%s\n</file>\n", filepath.ToSlash(relative), content)
		used += len(content)
		return nil
	})
	return context.String(), err
}

func isEditorSourceFile(path, language string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	switch strings.ToLower(language) {
	case "go":
		return extension == ".go"
	case "elixir":
		return extension == ".ex" || extension == ".exs"
	case "rust":
		return extension == ".rs"
	case "python":
		return extension == ".py"
	case "ruby":
		return extension == ".rb"
	case "javascript":
		return extension == ".js" || extension == ".jsx"
	case "typescript":
		return extension == ".ts" || extension == ".tsx"
	default:
		return false
	}
}
