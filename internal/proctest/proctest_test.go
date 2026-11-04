//go:build unix

package proctest

import (
	"errors"
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"
)

func startFixture(t *testing.T, survivors Survivors) (*GroupFixture, *exec.Cmd) {
	t.Helper()
	f := NewGroupFixture(t, survivors)
	cmd := exec.Command(f.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap the command whichever way it ends so it never lingers as a zombie.
	go func() { _ = cmd.Wait() }()
	return f, cmd
}

func TestGroupFixtureRecordsTermination(t *testing.T) {
	f, cmd := startFixture(t, Survivors{})
	if group := f.WaitReady(t); group != cmd.Process.Pid {
		t.Fatalf("WaitReady() = %d, want the command's process group %d", group, cmd.Process.Pid)
	}
	if got := f.Terminated(t); len(got) != 0 {
		t.Fatalf("Terminated() before any signal = %v", got)
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	f.WaitGone(t, 5*time.Second)
	if got := f.Terminated(t); !slices.Equal(got, Members) {
		t.Fatalf("Terminated() = %v, want %v", got, Members)
	}
}

// A fixture that survives SIGTERM must still be gone once its test ends.
func TestGroupFixtureCleanupKillsSurvivors(t *testing.T) {
	var group int
	t.Run("survives SIGTERM", func(t *testing.T) {
		f, cmd := startFixture(t, Survivors{Parent: true, Child: true})
		group = f.WaitReady(t)
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for !slices.Equal(f.Terminated(t), Members) {
			if time.Now().After(deadline) {
				t.Fatalf("Terminated() = %v, want %v", f.Terminated(t), Members)
			}
			time.Sleep(pollInterval)
		}
		if !f.groupAlive() {
			t.Fatal("fixture exited on SIGTERM although it survives it")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !errors.Is(syscall.Kill(-group, 0), syscall.ESRCH) {
		if time.Now().After(deadline) {
			t.Fatalf("process group %d outlived its test", group)
		}
		time.Sleep(pollInterval)
	}
}
