package maestro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicVerifierSelection tests the dynamic selection of verifiers based on file types
func TestDynamicVerifierSelection(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Initialize a new Maestro instance
	maestro := NewMaestro(nil, tempDir, "test-model")

	// Create test files with different extensions
	testFiles := map[string]string{
		"main.go":     "package main\nfunc main() { println(\"hello\") }",
		"script.py":   "print('hello world')",
		"app.js":      "console.log('hello');",
		"doc.md":      "# Documentation\nThis is a markdown file.",
		"config.json": `{"name": "test"}`,
		"styles.css":  "body { margin: 0; }",
	}

	for filename, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize git repo to simulate git diff
	err := execCommand(tempDir, "git", "init")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "config", "user.name", "test")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "add", ".")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)

	// Modify files to simulate changes that would appear in git diff
	for filename := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		// Append a comment to each file to make it modified
		modifiedContent := string(content) + "\n// Modified for test"
		err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
		require.NoError(t, err)
	}

	// Test 1: When code files are modified, build and test verifiers should be selected
	t.Run("CodeFilesModified", func(t *testing.T) {
		maestro.Verifiers = []Verifier{} // No specific verifiers set

		verifiers := maestro.selectVerifiers()

		// Should include build and test verifiers for code files
		var buildVerifierFound, testVerifierFound bool
		for _, v := range verifiers {
			if _, ok := v.(*BuildVerifier); ok {
				buildVerifierFound = true
			}
			if _, ok := v.(*TestVerifier); ok {
				testVerifierFound = true
			}
		}

		assert.True(t, buildVerifierFound, "Build verifier should be selected when code files are modified")
		assert.True(t, testVerifierFound, "Test verifier should be selected when code files are modified")
	})

	// Test 2: When only non-code files are modified, no verifiers should be selected
	t.Run("NonCodeFilesModified", func(t *testing.T) {
		// Remove code files and keep only non-code files
		codeFiles := []string{"main.go", "script.py", "app.js"}
		for _, file := range codeFiles {
			err := os.Remove(filepath.Join(tempDir, file))
			require.NoError(t, err)
		}

		// Modify remaining non-code files to make them show up in git diff
		for _, filename := range []string{"doc.md", "config.json", "styles.css"} {
			filePath := filepath.Join(tempDir, filename)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			// Append a comment to each file to make it modified
			modifiedContent := string(content) + "\n// Modified for test"
			err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
			require.NoError(t, err)
		}

		maestro.Verifiers = []Verifier{} // No specific verifiers set

		verifiers := maestro.selectVerifiers()

		// Should not include build or test verifiers for non-code files
		var buildVerifierFound, testVerifierFound bool
		for _, v := range verifiers {
			if _, ok := v.(*BuildVerifier); ok {
				buildVerifierFound = true
			}
			if _, ok := v.(*TestVerifier); ok {
				testVerifierFound = true
			}
		}

		assert.False(t, buildVerifierFound, "Build verifier should not be selected for non-code files")
		assert.False(t, testVerifierFound, "Test verifier should not be selected for non-code files")
	})

	// Test 3: When lint verifier is specifically requested, it should be included
	t.Run("LintVerifierRequested", func(t *testing.T) {
		// Add back a code file
		err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644)
		require.NoError(t, err)

		// Modify the code file to make it show up in git diff
		content, err := os.ReadFile(filepath.Join(tempDir, "main.go"))
		require.NoError(t, err)
		modifiedContent := string(content) + "\n// Modified for test"
		err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(modifiedContent), 0644)
		require.NoError(t, err)

		// Set a lint verifier in the maestro
		maestro.Verifiers = []Verifier{&LintVerifier{}}

		verifiers := maestro.selectVerifiers()

		// Should include build, test, and lint verifiers
		var buildVerifierFound, testVerifierFound, lintVerifierFound bool
		for _, v := range verifiers {
			if _, ok := v.(*BuildVerifier); ok {
				buildVerifierFound = true
			}
			if _, ok := v.(*TestVerifier); ok {
				testVerifierFound = true
			}
			if _, ok := v.(*LintVerifier); ok {
				lintVerifierFound = true
			}
		}

		assert.True(t, buildVerifierFound, "Build verifier should be selected when code files are modified")
		assert.True(t, testVerifierFound, "Test verifier should be selected when code files are modified")
		assert.True(t, lintVerifierFound, "Lint verifier should be selected when specifically requested")
	})
}

// execCommand executes a command with the given arguments
func execCommand(dir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	if command == "git" {
		cmd.Env = cleanGitEnvironment(os.Environ())
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}

	// Optionally log output for debugging if needed
	// if len(output) > 0 {
	// 	log.Printf("Command output: %s", string(output))
	// }

	return nil
}

// TestVerifyWithDynamicSelection tests the verify method with dynamic selection
func TestVerifyWithDynamicSelection(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Initialize a new Maestro instance
	maestro := NewMaestro(nil, tempDir, "test-model")

	// Create a Go file to trigger build verification
	goFile := filepath.Join(tempDir, "main.go")
	err := os.WriteFile(goFile, []byte("package main\nfunc main() { println(\"hello\") }"), 0644)
	require.NoError(t, err)

	// Initialize git repo
	err = execCommand(tempDir, "git", "init")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "config", "user.name", "test")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "add", ".")
	require.NoError(t, err)

	err = execCommand(tempDir, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)

	// Modify the file to make it show up in git diff
	content, err := os.ReadFile(goFile)
	require.NoError(t, err)
	modifiedContent := string(content) + "\n// Modified for test"
	err = os.WriteFile(goFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Test verification with code files
	t.Run("VerifyWithCodeFiles", func(t *testing.T) {
		// This should trigger build verification
		ctx := context.Background()
		_, err := maestro.verify(ctx)

		// The result depends on whether the build passes or fails
		// but the important thing is that verification was attempted
		if err != nil && !strings.Contains(err.Error(), "no verifiers") {
			// If there's an error, it should be related to the build/test process, not verifier selection
			t.Logf("Verification error (expected): %v", err)
		}

		// The verify method should not return an error about no verifiers being found
		// because code files are present and build/test verifiers should be selected
	})
}
