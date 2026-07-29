package maestro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEditorRepositoryContextIncludesImplementation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "store.go"), []byte("package session\ntype Store struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "store_test.go"), []byte("package session\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	context, err := buildEditorRepositoryContext(root, "Go", 32*1024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, `path="store.go"`) || !strings.Contains(context, "type Store struct") {
		t.Fatalf("implementation evidence missing: %s", context)
	}
	if strings.Contains(context, "store_test.go") {
		t.Fatalf("test file should not consume editor context: %s", context)
	}
}

func TestBuildPlanningRepositoryContextGroundsPlanInImplementation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ledger.go"), []byte("package ledger\nfunc Transfer() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ledger_test.go"), []byte("package ledger\nfunc TestTransferPreservesTotal() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "task.md"), []byte("Keep the public API stable.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	context, err := buildPlanningRepositoryContext(root, "Go", 32*1024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, `path="ledger.go"`) ||
		!strings.Contains(context, "func Transfer") ||
		!strings.Contains(context, "TestTransferPreservesTotal") ||
		!strings.Contains(context, "Keep the public API stable") {
		t.Fatalf("planning evidence missing: %s", context)
	}
}

func TestBuildRetryMessageIncludesCurrentRepositoryState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ledger.go"), []byte("package ledger\nvar attempt = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ledger_test.go"), []byte("package ledger\nfunc TestTransferContract() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "task.md"), []byte("Preserve the exported API and pass the race detector.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	message, err := buildRetryMessage(root, "Go", "go test failed")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "go test failed") ||
		!strings.Contains(message, "CURRENT REPOSITORY STATE") ||
		!strings.Contains(message, "var attempt = 2") ||
		!strings.Contains(message, "TestTransferContract") ||
		!strings.Contains(message, "Preserve the exported API") {
		t.Fatalf("retry message lacks current evidence: %s", message)
	}
}
