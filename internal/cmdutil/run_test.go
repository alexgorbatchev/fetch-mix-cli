//go:build unix

package cmdutil

import (
	"context"
	"testing"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/proctest"
	"github.com/alexgorbatchev/godeps/v2"
)

func runningCount() int {
	running.Lock()
	defer running.Unlock()
	return len(running.procs)
}

func TestRun(t *testing.T) {
	cmd := godeps.CommandContext(context.Background(), "sh", "-c", "printf ok")
	out := &stringWriter{}
	cmd.Stdout = out
	if err := Run(cmd); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.s != "ok" {
		t.Fatalf("Run() output = %q, want %q", out.s, "ok")
	}
	if n := runningCount(); n != 0 {
		t.Fatalf("%d commands still listed after Run returned", n)
	}
	if err := Run(godeps.CommandContext(context.Background(), "fetch-mix-missing-command")); err == nil {
		t.Fatal("Run() of a missing command succeeded")
	}
	if n := runningCount(); n != 0 {
		t.Fatalf("%d commands listed after a failed start", n)
	}
}

// A forced exit must not leave behind subprocesses that ignore SIGTERM.
func TestKillRunningKillsProcessGroups(t *testing.T) {
	fixture := proctest.NewGroupFixture(t, proctest.Survivors{Parent: true, Child: true})
	done := make(chan error, 1)
	go func() { done <- Run(godeps.CommandContext(context.Background(), fixture.Command)) }()
	fixture.WaitReady(t)

	KillRunning()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() of a killed command succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("KillRunning did not stop the command")
	}
	fixture.WaitGone(t, 5*time.Second)
	if got := fixture.Terminated(t); len(got) != 0 {
		t.Fatalf("processes that received SIGTERM = %v, want none", got)
	}
}

type stringWriter struct{ s string }

func (w *stringWriter) Write(p []byte) (int, error) {
	w.s += string(p)
	return len(p), nil
}
