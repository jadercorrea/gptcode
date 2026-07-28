//go:build windows

package processutil

import (
	"context"
	"os/exec"
)

func combinedOutput(ctx context.Context, command *exec.Cmd) ([]byte, error) {
	contextCommand := exec.CommandContext(ctx, command.Path, command.Args[1:]...)
	contextCommand.Dir = command.Dir
	return contextCommand.CombinedOutput()
}
