//go:build !unix

package cmdutil

import "os"

// killTree kills p; platforms without process groups have nothing more to
// signal.
func killTree(p *os.Process) {
	_ = p.Kill()
}
