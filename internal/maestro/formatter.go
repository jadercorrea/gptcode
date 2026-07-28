package maestro

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func formatModifiedGoFiles(ctx context.Context, root string, modifiedFiles []string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving repository root: %w", err)
	}
	absoluteRoot, err = filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", fmt.Errorf("resolving repository root symlinks: %w", err)
	}
	var paths []string
	seen := make(map[string]struct{}, len(modifiedFiles))
	for _, modified := range modifiedFiles {
		path := modified
		if !filepath.IsAbs(path) {
			path = filepath.Join(absoluteRoot, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolving modified path: %w", err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("resolving modified path symlinks: %w", err)
		}
		relative, err := filepath.Rel(absoluteRoot, path)
		if err != nil {
			return "", fmt.Errorf("relativizing modified path: %w", err)
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", errors.New("modified file is outside repository root")
		}
		if filepath.Ext(relative) != ".go" {
			continue
		}
		if _, found := seen[relative]; found {
			continue
		}
		seen[relative] = struct{}{}
		paths = append(paths, relative)
	}
	if len(paths) == 0 {
		return "", nil
	}

	command := exec.CommandContext(ctx, "gofmt", append([]string{"-w"}, paths...)...)
	command.Dir = absoluteRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("gofmt modified files: %w", err)
	}
	return string(output), nil
}
