//go:build e2e

package run_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jadercorrea/gptcode/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestExecutor(t *testing.T) {
	t.Run("run go tests", func(t *testing.T) {
		tempDir := t.TempDir()

		setupGitRepo(t, tempDir)

		mainGo := filepath.Join(tempDir, "main.go")
		require.NoError(t, os.WriteFile(mainGo, []byte(`package main

func Add(a, b int) int {
	return a + b
}

func main() {}
`), 0644))

		testGo := filepath.Join(tempDir, "main_test.go")
		require.NoError(t, os.WriteFile(testGo, []byte(`package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("2 + 3 should equal 5")
	}
}
`), 0644))

		goMod := filepath.Join(tempDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte(`module testproject

go 1.21
`), 0644))

		executor := validation.NewTestExecutor(tempDir)
		result, err := executor.RunTests()

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success, "Tests should pass")
		assert.Greater(t, result.Passed, 0, "Should have passing tests")
		assert.Equal(t, 0, result.Failed, "Should have no failures")
	})

	t.Run("detect failing go tests", func(t *testing.T) {
		tempDir := t.TempDir()

		setupGitRepo(t, tempDir)

		mainGo := filepath.Join(tempDir, "main.go")
		require.NoError(t, os.WriteFile(mainGo, []byte(`package main

func Add(a, b int) int {
	return a + b
}

func main() {}
`), 0644))

		testGo := filepath.Join(tempDir, "main_test.go")
		require.NoError(t, os.WriteFile(testGo, []byte(`package main

import "testing"

func TestAddFailing(t *testing.T) {
	if Add(2, 3) != 10 {
		t.Error("This test will fail")
	}
}
`), 0644))

		goMod := filepath.Join(tempDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte(`module testproject

go 1.21
`), 0644))

		executor := validation.NewTestExecutor(tempDir)
		result, err := executor.RunTests()

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success, "Tests should fail")
		assert.Greater(t, result.Failed, 0, "Should have failing tests")
	})
}

func TestLinterExecutor(t *testing.T) {
	t.Run("run go linters", func(t *testing.T) {
		tempDir := t.TempDir()

		setupGitRepo(t, tempDir)

		mainGo := filepath.Join(tempDir, "main.go")
		require.NoError(t, os.WriteFile(mainGo, []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`), 0644))

		goMod := filepath.Join(tempDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte(`module testproject

go 1.21
`), 0644))

		executor := validation.NewLinterExecutor(tempDir)
		results, err := executor.RunLinters()

		require.NoError(t, err)
		assert.NotEmpty(t, results, "Should have at least one linter result")

		for _, result := range results {
			assert.NotEmpty(t, result.Tool, "Tool name should be set")
			t.Logf("Linter: %s, Success: %v, Issues: %d", result.Tool, result.Success, result.Issues)
		}
	})

	t.Run("detect linter issues", func(t *testing.T) {
		tempDir := t.TempDir()

		setupGitRepo(t, tempDir)

		mainGo := filepath.Join(tempDir, "main.go")
		require.NoError(t, os.WriteFile(mainGo, []byte(`package main

import "fmt"

func unused() {
	x := 42
}

func main() {
	fmt.Println("Hello")
}
`), 0644))

		goMod := filepath.Join(tempDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte(`module testproject

go 1.21
`), 0644))

		executor := validation.NewLinterExecutor(tempDir)
		results, err := executor.RunLinters()

		require.NoError(t, err)
		assert.NotEmpty(t, results)

		hasIssues := false
		for _, result := range results {
			if !result.Success || result.Issues > 0 {
				hasIssues = true
				t.Logf("Found issues in %s: %d", result.Tool, result.Issues)
			}
		}

		assert.True(t, hasIssues, "Should detect unused variable")
	})
}

func TestValidationWorkflow(t *testing.T) {
	t.Run("full validation workflow", func(t *testing.T) {
		tempDir := t.TempDir()

		setupGitRepo(t, tempDir)

		mainGo := filepath.Join(tempDir, "main.go")
		require.NoError(t, os.WriteFile(mainGo, []byte(`package main

func Add(a, b int) int {
	return a + b
}

func main() {}
`), 0644))

		testGo := filepath.Join(tempDir, "main_test.go")
		require.NoError(t, os.WriteFile(testGo, []byte(`package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("2 + 3 should equal 5")
	}
}
`), 0644))

		goMod := filepath.Join(tempDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte(`module testproject

go 1.21
`), 0644))

		testExec := validation.NewTestExecutor(tempDir)
		testResult, err := testExec.RunTests()
		require.NoError(t, err)
		assert.True(t, testResult.Success, "Tests should pass")

		lintExec := validation.NewLinterExecutor(tempDir)
		lintResults, err := lintExec.RunLinters()
		require.NoError(t, err)
		assert.NotEmpty(t, lintResults, "Should have linter results")

		allLintsPassed := true
		for _, result := range lintResults {
			if !result.Success {
				allLintsPassed = false
				t.Logf("Linter %s failed: %s", result.Tool, result.ErrorMessage)
			}
		}

		overallSuccess := testResult.Success && allLintsPassed
		t.Logf("Overall validation success: %v", overallSuccess)
	})
}
