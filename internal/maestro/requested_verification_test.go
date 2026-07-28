package maestro

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunRequestedVerificationCancelsProcessTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group behavior")
	}
	root := t.TempDir()
	pidFile := filepath.Join(root, "child.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := runRequestedVerification(ctx, root, []string{
		"sh", "-c", "sleep 30 & echo $! > child.pid; wait",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	data, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid := strings.TrimSpace(string(data))
	if processExists(pid) {
		t.Fatalf("child process %s survived cancellation", pid)
	}
}

func processExists(pid string) bool {
	return exec.Command("sh", "-c", "kill -0 "+pid).Run() == nil
}

func TestRequestedVerificationCommandUsesExplicitRaceCheck(t *testing.T) {
	got := requestedVerificationCommand("Fix it. Verify with go test -race ./...")
	want := []string{"go", "test", "-race", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}

func TestRequestedVerificationCommandUsesScopedGoCheck(t *testing.T) {
	got := requestedVerificationCommand("Fix it. Run go test ./internal/tools and report exact evidence.")
	want := []string{"go", "test", "./internal/tools"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}

func TestRequestedVerificationCommandUsesScopedMixCheck(t *testing.T) {
	got := requestedVerificationCommand(
		"Fix it. Run mix test test/teiserver_web/components/core_components_test.exs and report exact evidence.",
	)
	want := []string{"mix", "test", "test/teiserver_web/components/core_components_test.exs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}

func TestRequestedVerificationCommandRejectsShellSyntax(t *testing.T) {
	got := requestedVerificationCommand("Run go test ./...; rm -rf /")
	if got != nil {
		t.Fatalf("command = %v, want nil", got)
	}
}

func TestRepositoryVerificationCommandPrefersMakeVerify(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("verify:\n\t@go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := repositoryVerificationCommand(root)
	want := []string{"make", "verify"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}

func TestRepositoryVerificationCommandFallsBackToGoTests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/verify\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := repositoryVerificationCommand(root)
	want := []string{"go", "test", "./..."}
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
