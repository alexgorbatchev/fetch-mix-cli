package cmdutil

import (
	"os"
	"os/exec"
	"sync"
)

// running holds the processes started by Run that have not exited yet.
var running = struct {
	sync.Mutex
	procs map[*os.Process]struct{}
}{procs: make(map[*os.Process]struct{})}

// Run starts cmd and waits for it, listing it for KillRunning while it runs.
// cmd comes from godeps.CommandContext, which on Unix starts it in its own
// process group.
func Run(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	running.Lock()
	running.procs[cmd.Process] = struct{}{}
	running.Unlock()
	defer func() {
		running.Lock()
		delete(running.procs, cmd.Process)
		running.Unlock()
	}()
	return cmd.Wait()
}

// KillRunning immediately kills every command Run is waiting for, including
// the rest of its process group on Unix. A forced exit calls it because
// exiting also ends the grace period after which godeps would kill
// subprocesses that ignore SIGTERM.
func KillRunning() {
	running.Lock()
	defer running.Unlock()
	for p := range running.procs {
		killTree(p)
	}
}
