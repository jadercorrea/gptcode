package processutil

import (
	"context"
	"os/exec"
)

// CombinedOutput runs a command and ensures cancellation is delegated to the
// platform implementation, which may need to terminate an entire process tree.
func CombinedOutput(ctx context.Context, cwd, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = cwd
	return combinedOutput(ctx, command)
}
