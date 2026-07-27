package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/refactor"
)

func TestSignatureRefactor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signature-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

import "fmt"

func processData(data string) string {
	return fmt.Sprintf("processed: %s", data)
}

func main() {
	result := processData("test")
	fmt.Println(result)
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	provider := &agents.MockProvider{
		Response: `package main

import (
	"context"
	"fmt"
)

func processData(ctx context.Context, data string) (string, error) {
	return fmt.Sprintf("processed: %s", data), nil
}

func main() {
	result, err := processData(context.Background(), "test")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result)
}`,
	}
	refactorTool := refactor.NewSignatureRefactor(provider, "test-model", tmpDir)

	newSig := "(ctx context.Context, data string) (string, error)"

	result, err := refactorTool.RefactorSignature(context.Background(), "processData", newSig)
	if err != nil {
		t.Fatalf("RefactorSignature failed: %v", err)
	}

	if result.Function != "processData" {
		t.Errorf("expected function 'processData', got %s", result.Function)
	}

	if !strings.Contains(result.OldSignature, "func processData(data string) string") {
		t.Errorf("unexpected old signature: %s", result.OldSignature)
	}

	if len(result.UpdatedFiles) == 0 {
		t.Error("expected at least one updated file")
	}

	if len(result.Errors) > 0 {
		t.Logf("refactoring completed with errors: %v", result.Errors)
	}

	mainUpdated, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("failed to read updated main.go: %v", err)
	}

	mainStr := string(mainUpdated)
	t.Logf("Updated main.go content:\n%s", mainStr)
	if !strings.Contains(mainStr, "context.Context") {
		t.Error("main.go should contain context.Context after refactoring")
	}
}
