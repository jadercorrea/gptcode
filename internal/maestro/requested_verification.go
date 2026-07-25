package maestro

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func requestedVerificationCommand(task string) []string {
	lower := strings.ToLower(task)
	switch {
	case strings.Contains(lower, "go test -race ./..."):
		return []string{"go", "test", "-race", "./..."}
	case strings.Contains(lower, "go test ./..."):
		return []string{"go", "test", "./..."}
	default:
		return nil
	}
}

func runRequestedVerification(ctx context.Context, cwd string, command []string) (string, error) {
	if len(command) == 0 {
		return "", nil
	}
	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Dir = cwd
	output, err := process.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w", strings.Join(command, " "), err)
	}
	return string(output), nil
}
