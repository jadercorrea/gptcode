package maestro

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRequestedVerificationCommandUsesExplicitRaceCheck(t *testing.T) {
	got := requestedVerificationCommand("Fix it. Verify with go test -race ./...")
	want := []string{"go", "test", "-race", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}

func TestRunRequestedVerification(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/verify\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "verify_test.go"), []byte("package verify\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runRequestedVerification(context.Background(), root, []string{"go", "test", "./..."})
	if err != nil {
		t.Fatalf("verification failed: %v\n%s", err, output)
	}
}
