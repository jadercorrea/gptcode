//go:build !windows

package processutil

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
)

func combinedOutput(ctx context.Context, command *exec.Cmd) ([]byte, error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		return output.Bytes(), err
	}

	var waitErr error
	done := make(chan struct{})
	go func() {
		waitErr = command.Wait()
		close(done)
	}()

	select {
	case <-done:
		return output.Bytes(), waitErr
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-done
		return output.Bytes(), ctx.Err()
	}
}
