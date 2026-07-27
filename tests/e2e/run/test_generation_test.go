//go:build e2e

package run_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/mockgen"
	"github.com/jadercorrea/gptcode/internal/testgen"
)

func TestTestGeneration(t *testing.T) {
	t.Run("generate unit tests", func(t *testing.T) {
		if os.Getenv("SKIP_E2E_LLM") != "" {
			t.Skip("Skipping LLM-dependent E2E test")
		}

		tmpDir := t.TempDir()

		// Create a simple Go file to test
		sourceCode := `package calculator

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference between two integers
func Subtract(a, b int) int {
	return a - b
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	return a * b
}

// Divide returns the quotient of two integers and an error if dividing by zero
func Divide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}
`

		sourceFile := filepath.Join(tmpDir, "calculator.go")
		if err := os.WriteFile(sourceFile, []byte(sourceCode), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		// Create go.mod so it's a valid Go module
		goMod := `module testcalc

go 1.21
`
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("Failed to create go.mod: %v", err)
		}

		// Load config and create generator
		setup, err := config.LoadSetup()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		backendName := setup.Defaults.Backend
		if backendName == "" {
			backendName = "anthropic"
		}
		backendCfg := setup.Backend[backendName]

		var provider llm.Provider
		if backendCfg.Type == "ollama" {
			provider = llm.NewOllama(backendCfg.BaseURL)
		} else {
			provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}

		queryModel := backendCfg.GetModelForAgent("query")
		if queryModel == "" {
			queryModel = backendCfg.DefaultModel
		}

		generator, err := testgen.NewTestGenerator(provider, queryModel, tmpDir)
		if err != nil {
			t.Fatalf("Failed to create test generator: %v", err)
		}

		// Generate tests
		t.Log("Generating unit tests...")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		result, err := generator.GenerateUnitTests(ctx, "calculator.go")
		if err != nil && result == nil {
			t.Fatalf("Failed to generate tests: %v", err)
		}

		// Verify test file was created
		testFile := filepath.Join(tmpDir, result.TestFile)
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Fatalf("Test file was not created: %s", result.TestFile)
		}

		// Verify test file has content
		testContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}

		if len(testContent) == 0 {
			t.Fatal("Test file is empty")
		}

		testStr := string(testContent)
		t.Logf("Generated test file (%d bytes)", len(testContent))

		// Verify basic test structure
		if !strings.Contains(testStr, "package calculator") {
			t.Error("Test file missing package declaration")
		}
		if !strings.Contains(testStr, "func Test") {
			t.Error("Test file missing test functions")
		}
		if !strings.Contains(testStr, "*testing.T") {
			t.Error("Test file missing testing.T parameter")
		}

		// Try to compile the test
		t.Log("Validating test compiles...")
		cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", ".")
		cmd.Dir = tmpDir
		output, compileErr := cmd.CombinedOutput()

		if compileErr != nil {
			t.Logf("Compilation output:\n%s", string(output))
			t.Errorf("Generated test does not compile: %v", compileErr)
		} else {
			t.Log("✓ Test compiles successfully")
		}

		// Try to run the test
		if compileErr == nil {
			t.Log("Running generated tests...")
			cmd = exec.Command("go", "test", "-v", ".")
			cmd.Dir = tmpDir
			output, runErr := cmd.CombinedOutput()
			t.Logf("Test output:\n%s", string(output))

			if runErr != nil {
				t.Logf("⚠️  Tests failed to run: %v (this is OK if logic is wrong, not if syntax is wrong)", runErr)
			} else {
				t.Log("✓ Tests ran successfully")
			}
		}

		// Overall validation
		if result.Valid {
			t.Log("✓ Test generation succeeded with valid output")
		} else {
			t.Logf("⚠️  Test generation completed but validation failed: %v", result.Error)
		}
	})

	t.Run("generate integration tests", func(t *testing.T) {
		t.Skip("TODO: Implement - Test interaction between components")
	})

	t.Run("add missing test coverage", func(t *testing.T) {
		t.Skip("TODO: Implement - Identify untested paths")
	})

	t.Run("generate mocks for interfaces", func(t *testing.T) {
		if os.Getenv("SKIP_E2E_LLM") != "" {
			t.Skip("Skipping LLM-dependent E2E test")
		}

		tmpDir := t.TempDir()

		// Create a simple interface file
		interfaceCode := `package storage

import "context"

// Repository defines storage operations
type Repository interface {
	// Get retrieves an item by ID
	Get(ctx context.Context, id string) (*Item, error)
	// Save stores an item
	Save(ctx context.Context, item *Item) error
	// Delete removes an item
	Delete(ctx context.Context, id string) error
	// List returns all items
	List(ctx context.Context) ([]*Item, error)
}

// Item represents a stored object
type Item struct {
	ID   string
	Name string
}
`

		sourceFile := filepath.Join(tmpDir, "repository.go")
		if err := os.WriteFile(sourceFile, []byte(interfaceCode), 0644); err != nil {
			t.Fatalf("Failed to create interface file: %v", err)
		}

		// Create go.mod
		goMod := `module testrepo

go 1.21
`
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("Failed to create go.mod: %v", err)
		}

		// Load config and create generator
		setup, err := config.LoadSetup()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		backendName := setup.Defaults.Backend
		if backendName == "" {
			backendName = "anthropic"
		}
		backendCfg := setup.Backend[backendName]

		var provider llm.Provider
		if backendCfg.Type == "ollama" {
			provider = llm.NewOllama(backendCfg.BaseURL)
		} else {
			provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
		}

		queryModel := backendCfg.GetModelForAgent("query")
		if queryModel == "" {
			queryModel = backendCfg.DefaultModel
		}

		generator := mockgen.NewMockGenerator(provider, queryModel, tmpDir)

		// Generate mock
		t.Log("Generating mock...")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		result, err := generator.GenerateMock(ctx, "repository.go")
		if err != nil && result == nil {
			t.Fatalf("Failed to generate mock: %v", err)
		}

		// Verify mock file was created
		mockFile := filepath.Join(tmpDir, result.MockFile)
		if _, err := os.Stat(mockFile); os.IsNotExist(err) {
			t.Fatalf("Mock file was not created: %s", result.MockFile)
		}

		// Verify mock file has content
		mockContent, err := os.ReadFile(mockFile)
		if err != nil {
			t.Fatalf("Failed to read mock file: %v", err)
		}

		if len(mockContent) == 0 {
			t.Fatal("Mock file is empty")
		}

		mockStr := string(mockContent)
		t.Logf("Generated mock file (%d bytes)", len(mockContent))

		// Verify basic mock structure
		if !strings.Contains(mockStr, "package storage") {
			t.Error("Mock file missing package declaration")
		}
		if !strings.Contains(mockStr, "Mock") {
			t.Error("Mock file missing mock struct")
		}
		if !strings.Contains(mockStr, "Get") || !strings.Contains(mockStr, "Save") {
			t.Error("Mock file missing interface methods")
		}

		// Try to compile the mock
		t.Log("Validating mock compiles...")
		cmd := exec.Command("go", "build", ".")
		cmd.Dir = tmpDir
		output, compileErr := cmd.CombinedOutput()

		if compileErr != nil {
			t.Logf("Compilation output:\n%s", string(output))
			t.Errorf("Generated mock does not compile: %v", compileErr)
		} else {
			t.Log("✓ Mock compiles successfully")
		}

		if result.Valid {
			t.Log("✓ Mock generation succeeded")
		} else {
			t.Logf("⚠️  Mock generation completed but validation failed: %v", result.Error)
		}
	})

	t.Run("snapshot testing", func(t *testing.T) {
		t.Skip("TODO: Implement - Generate and update snapshots")
	})
}
