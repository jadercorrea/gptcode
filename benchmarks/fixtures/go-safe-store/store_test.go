package safestore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStoresNestedFileInsideRoot(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.Write("reports/summary.txt", []byte("verified\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "reports", "summary.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "verified\n" {
		t.Fatalf("content = %q", content)
	}
}

func TestWriteAllowsDotsInsidePathSegment(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	name := "reports/v1..v2.txt"
	if err := store.Write(name, []byte("valid\n")); err != nil {
		t.Fatalf("Write() error = %v for legitimate name", err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "valid\n" {
		t.Fatalf("content = %q", content)
	}
}

func TestWriteRejectsTraversal(t *testing.T) {
	parent := t.TempDir()
	parent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "store")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store := New(root)

	err = store.Write("../escaped.txt", []byte("escape"))
	if err == nil {
		t.Fatal("Write() error = nil, want traversal rejection")
	}
	if _, statErr := os.Stat(filepath.Join(parent, "escaped.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("file escaped root: %v", statErr)
	}
}

func TestWriteRejectsTraversalAfterNormalizingSegments(t *testing.T) {
	parent := t.TempDir()
	parent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "store")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store := New(root)

	err = store.Write("reports/../../escaped.txt", []byte("escape"))
	if err == nil {
		t.Fatal("Write() error = nil, want normalized traversal rejection")
	}
	if _, statErr := os.Stat(filepath.Join(parent, "escaped.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("file escaped root: %v", statErr)
	}
}

func TestWriteRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.Write(filepath.Join(t.TempDir(), "escaped.txt"), []byte("escape")); err == nil {
		t.Fatal("Write() error = nil, want absolute path rejection")
	}
}

func TestWriteRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	store := New(root)

	err := store.Write("linked/escaped.txt", []byte("escape"))
	if err == nil {
		t.Fatal("Write() error = nil, want symlink rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "escaped.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("file escaped through symlink: %v", statErr)
	}
}
