//go:build windows

package cmdutil

import (
	"context"
	"os/exec"
)

// NewCommand creates an exec.Cmd configured for process termination on context cancellation.
func NewCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	return cmd
}
