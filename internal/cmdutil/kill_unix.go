//go:build unix

package cmdutil

import (
	"os"
	"syscall"
)

// killTree sends SIGKILL to the process group that p leads.
func killTree(p *os.Process) {
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}
