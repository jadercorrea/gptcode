package maestro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatModifiedGoFilesFormatsOnlyRepositoryFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "store.go")
	if err := os.WriteFile(path, []byte("package store\n\nfunc Value( )int{return 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := formatModifiedGoFiles(context.Background(), root, []string{"store.go"}); err != nil {
		t.Fatalf("formatModifiedGoFiles() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package store\n\nfunc Value() int { return 1 }\n" {
		t.Fatalf("formatted content = %q", content)
	}
}

func TestFormatModifiedGoFilesRejectsPathOutsideRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := formatModifiedGoFiles(context.Background(), root, []string{outside}); err == nil {
		t.Fatal("formatModifiedGoFiles() error = nil, want outside path rejection")
	}
}

func TestFormatModifiedGoFilesRejectsSymlinkOutsideRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n\nfunc Value( )int{return 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}

	if _, err := formatModifiedGoFiles(context.Background(), root, []string{"linked.go"}); err == nil {
		t.Fatal("formatModifiedGoFiles() error = nil, want symlink escape rejection")
	}
	content, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package outside\n\nfunc Value( )int{return 1}\n" {
		t.Fatal("formatter changed file outside repository")
	}
}
