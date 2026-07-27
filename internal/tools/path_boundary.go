package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveRepositoryPath(workdir, requestedPath string, allowMissing bool) (string, error) {
	if filepath.IsAbs(requestedPath) {
		return "", fmt.Errorf("path must be relative to the repository")
	}

	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository root symlinks: %w", err)
	}

	candidate := filepath.Join(root, filepath.Clean(requestedPath))
	if !pathWithinRoot(root, candidate) {
		return "", fmt.Errorf("path escapes repository boundary")
	}

	resolved, err := filepath.EvalSymlinks(candidate)
	if err == nil {
		if !pathWithinRoot(root, resolved) {
			return "", fmt.Errorf("path escapes repository boundary through a symlink")
		}
		return resolved, nil
	}
	if !allowMissing || !os.IsNotExist(err) {
		return "", err
	}

	ancestor := candidate
	for {
		ancestor = filepath.Dir(ancestor)
		if !pathWithinRoot(root, ancestor) {
			return "", fmt.Errorf("path escapes repository boundary")
		}

		resolvedAncestor, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			if !pathWithinRoot(root, resolvedAncestor) {
				return "", fmt.Errorf("path escapes repository boundary through a symlink")
			}
			return candidate, nil
		}
		if !os.IsNotExist(resolveErr) {
			return "", resolveErr
		}
		if ancestor == root {
			return "", resolveErr
		}
	}
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
