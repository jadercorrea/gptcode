package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectMap(t *testing.T) {
	t.Run("basic structure", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gptcode_test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.go"), []byte("package sub"), 0644)

		call := ToolCall{
			Name: "project_map",
			Arguments: map[string]interface{}{
				"max_depth": float64(3),
			},
		}

		result := ProjectMap(call, tmpDir)
		if result.Error != "" {
			t.Fatalf("ProjectMap failed: %s", result.Error)
		}

		if !strings.Contains(result.Result, "📄 file1.go") {
			t.Error("ProjectMap missing file1.go")
		}
		if !strings.Contains(result.Result, "📂 subdir/") {
			t.Error("ProjectMap missing subdir/")
		}
	})

	t.Run("filters ignored directories", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gptcode_test_filter")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
		os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755)
		os.Mkdir(filepath.Join(tmpDir, ".git"), 0755)
		os.Mkdir(filepath.Join(tmpDir, "src"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "node_modules", "lib.js"), []byte("code"), 0644)

		call := ToolCall{
			Name: "project_map",
			Arguments: map[string]interface{}{
				"max_depth": float64(3),
			},
		}

		result := ProjectMap(call, tmpDir)
		if result.Error != "" {
			t.Fatalf("ProjectMap failed: %s", result.Error)
		}

		if strings.Contains(result.Result, "node_modules") {
			t.Error("ProjectMap should filter node_modules")
		}
		if strings.Contains(result.Result, "vendor") {
			t.Error("ProjectMap should filter vendor")
		}
		if strings.Contains(result.Result, ".git") {
			t.Error("ProjectMap should filter .git")
		}
		if !strings.Contains(result.Result, "📂 src/") {
			t.Error("ProjectMap should include src/")
		}
	})

	t.Run("respects max_depth", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gptcode_test_depth")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "a", "b", "c", "deep.go"), []byte("package deep"), 0644)

		call := ToolCall{
			Name: "project_map",
			Arguments: map[string]interface{}{
				"max_depth": float64(2),
			},
		}

		result := ProjectMap(call, tmpDir)
		if result.Error != "" {
			t.Fatalf("ProjectMap failed: %s", result.Error)
		}

		if strings.Contains(result.Result, "deep.go") {
			t.Error("ProjectMap should not include files beyond max_depth")
		}
	})
}

func TestApplyPatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gptcode_patch_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"
	os.WriteFile(filePath, []byte(content), 0644)

	t.Run("exact match", func(t *testing.T) {
		call := ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "test.txt",
				"search":  "line2\n",
				"replace": "line2_modified\n",
			},
		}

		result := ApplyPatch(call, tmpDir)
		if result.Error != "" {
			t.Fatalf("ApplyPatch failed: %s", result.Error)
		}

		newContent, _ := os.ReadFile(filePath)
		if string(newContent) != "line1\nline2_modified\nline3\n" {
			t.Errorf("Patch not applied correctly. Got: %s", string(newContent))
		}
	})

	os.WriteFile(filePath, []byte(content), 0644)

	t.Run("fuzzy match with whitespace", func(t *testing.T) {
		call := ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "test.txt",
				"search":  "  line2  \n",
				"replace": "line2_fuzzy\n",
			},
		}

		result := ApplyPatch(call, tmpDir)
		if result.Error != "" {
			t.Fatalf("Fuzzy match failed: %s", result.Error)
		}

		newContent, _ := os.ReadFile(filePath)
		if !strings.Contains(string(newContent), "line2_fuzzy") {
			t.Errorf("Fuzzy patch not applied. Got: %s", string(newContent))
		}
	})

	t.Run("search not found", func(t *testing.T) {
		call := ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "test.txt",
				"search":  "nonexistent\n",
				"replace": "foo\n",
			},
		}

		result := ApplyPatch(call, tmpDir)
		if result.Error == "" {
			t.Error("Expected error for nonexistent search block")
		}
	})

	t.Run("empty search", func(t *testing.T) {
		call := ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "test.txt",
				"search":  "",
				"replace": "foo\n",
			},
		}

		result := ApplyPatch(call, tmpDir)
		if result.Error == "" {
			t.Error("Expected error for empty search block")
		}
	})

	t.Run("missing parameters", func(t *testing.T) {
		call := ToolCall{
			Name:      "apply_patch",
			Arguments: map[string]interface{}{},
		}

		result := ApplyPatch(call, tmpDir)
		if result.Error == "" {
			t.Error("Expected error for missing parameters")
		}
	})
}

func TestPathSecurity(t *testing.T) {
	parent := t.TempDir()
	workDir := filepath.Join(parent, "repository")
	sentinelDir := filepath.Join(parent, "outside")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatalf("Failed to create workDir: %v", err)
	}
	if err := os.Mkdir(sentinelDir, 0755); err != nil {
		t.Fatalf("Failed to create sentinel dir: %v", err)
	}

	sentinelFile := filepath.Join(sentinelDir, "sentinel.txt")
	if err := os.WriteFile(sentinelFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to write sentinel file: %v", err)
	}

	// Create a symlink from inside workDir to the outside sentinel file
	symlinkPath := filepath.Join(workDir, "symlink_to_secret")
	if err := os.Symlink(sentinelFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}
	symlinkDir := filepath.Join(workDir, "symlink_to_outside")
	if err := os.Symlink(sentinelDir, symlinkDir); err != nil {
		t.Fatalf("Failed to create directory symlink: %v", err)
	}

	// --- Test cases ---

	t.Run("read_file path traversal", func(t *testing.T) {
		// Attempt to read sentinel file via path traversal
		relativePath, err := filepath.Rel(workDir, sentinelFile)
		if err != nil {
			t.Fatalf("Failed to calculate relative path: %v", err)
		}

		call := ToolCall{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": relativePath,
			},
		}

		// This should fail because the path escapes the workDir
		result := readFile(call, workDir)
		if result.Error == "" {
			t.Errorf("read_file allowed path traversal via '%s'", relativePath)
		}
	})

	t.Run("read_file symlink escape", func(t *testing.T) {
		// Attempt to read sentinel file via symlink
		call := ToolCall{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": "symlink_to_secret",
			},
		}

		// This should fail because the symlink points outside the workDir
		result := readFile(call, workDir)
		if result.Error == "" {
			t.Errorf("read_file allowed symlink escape via 'symlink_to_secret'")
		}
	})

	t.Run("list_files path traversal", func(t *testing.T) {
		call := ToolCall{
			Name: "list_files",
			Arguments: map[string]interface{}{
				"path": "../outside",
			},
		}

		// This should fail because the path escapes the workDir
		result := listFiles(call, workDir)
		if result.Error == "" {
			t.Errorf("list_files allowed path traversal via '../outside'")
		}
	})

	t.Run("write_file path traversal", func(t *testing.T) {
		if err := os.WriteFile(sentinelFile, []byte("secret"), 0644); err != nil {
			t.Fatalf("Failed to reset sentinel file: %v", err)
		}
		relativePath, err := filepath.Rel(workDir, sentinelFile)
		if err != nil {
			t.Fatalf("Failed to calculate relative path: %v", err)
		}

		call := ToolCall{
			Name: "write_file",
			Arguments: map[string]interface{}{
				"path":    relativePath,
				"content": "overwrite",
			},
		}
		result := writeFile(call, workDir)
		if result.Error == "" {
			t.Errorf("write_file allowed path traversal via '%s'", relativePath)
		}
		content, err := os.ReadFile(sentinelFile)
		if err != nil {
			t.Fatalf("Failed to read sentinel after rejected write: %v", err)
		}
		if string(content) != "secret" {
			t.Fatalf("write_file modified the outside sentinel: %q", content)
		}
	})

	t.Run("write_file rejects a missing file below an escaping symlink", func(t *testing.T) {
		result := writeFile(ToolCall{
			Name: "write_file",
			Arguments: map[string]interface{}{
				"path":    "symlink_to_outside/created.txt",
				"content": "outside",
			},
		}, workDir)
		if result.Error == "" {
			t.Error("write_file allowed creation below an escaping symlink")
		}
		if _, err := os.Stat(filepath.Join(sentinelDir, "created.txt")); !os.IsNotExist(err) {
			t.Fatalf("write_file created an outside file: %v", err)
		}
	})

	t.Run("apply_patch symlink escape", func(t *testing.T) {
		if err := os.WriteFile(sentinelFile, []byte("secret"), 0644); err != nil {
			t.Fatalf("Failed to reset sentinel file: %v", err)
		}
		call := ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "symlink_to_secret",
				"search":  "secret",
				"replace": "hacked",
			},
		}
		result := ApplyPatch(call, workDir)
		if result.Error == "" {
			t.Errorf("apply_patch allowed symlink escape")
		}
		content, err := os.ReadFile(sentinelFile)
		if err != nil {
			t.Fatalf("Failed to read sentinel after rejected patch: %v", err)
		}
		if string(content) != "secret" {
			t.Fatalf("apply_patch modified the outside sentinel: %q", content)
		}
	})

	t.Run("apply_patch path traversal", func(t *testing.T) {
		if err := os.WriteFile(sentinelFile, []byte("secret"), 0644); err != nil {
			t.Fatalf("Failed to reset sentinel file: %v", err)
		}
		result := ApplyPatch(ToolCall{
			Name: "apply_patch",
			Arguments: map[string]interface{}{
				"path":    "../outside/sentinel.txt",
				"search":  "secret",
				"replace": "hacked",
			},
		}, workDir)
		if result.Error == "" {
			t.Error("apply_patch allowed path traversal")
		}
		content, err := os.ReadFile(sentinelFile)
		if err != nil {
			t.Fatalf("Failed to read sentinel after rejected patch: %v", err)
		}
		if string(content) != "secret" {
			t.Fatalf("apply_patch modified the outside sentinel: %q", content)
		}
	})

	t.Run("read_file absolute path", func(t *testing.T) {
		result := readFile(ToolCall{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": sentinelFile,
			},
		}, workDir)
		if result.Error == "" {
			t.Errorf("read_file allowed absolute path outside workdir")
		}
	})

	t.Run("normal paths remain available", func(t *testing.T) {
		result := writeFile(ToolCall{
			Name: "write_file",
			Arguments: map[string]interface{}{
				"path":    "nested/inside.txt",
				"content": "inside",
			},
		}, workDir)
		if result.Error != "" {
			t.Fatalf("write_file rejected an in-repository path: %s", result.Error)
		}

		result = readFile(ToolCall{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": "nested/inside.txt",
			},
		}, workDir)
		if result.Error != "" || result.Result != "inside" {
			t.Fatalf("read_file failed for an in-repository path: %+v", result)
		}
	})
}
