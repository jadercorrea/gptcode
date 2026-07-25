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
