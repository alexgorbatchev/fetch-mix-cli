//go:build unix

// Package proctest provides fake external commands for tests that cancel a
// running subprocess and check how its process group was stopped.
package proctest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	pollInterval = 20 * time.Millisecond
	readyTimeout = 10 * time.Second
)

// Members are the processes a GroupFixture command runs: the command itself
// and a background child in the same process group.
var Members = []string{"child", "parent"}

// GroupFixture is a fake command that runs as two processes, the command
// itself and a background child. Both record each SIGTERM they receive.
type GroupFixture struct {
	// Command is the absolute path of the executable to run.
	Command string
	marks   string
	group   int
}

// NewGroupFixture writes the fake command into a test-owned directory. When
// survivesTerm is true, both processes keep running after SIGTERM, so only
// SIGKILL can stop them; otherwise they exit on SIGTERM.
func NewGroupFixture(t testing.TB, survivesTerm bool) *GroupFixture {
	t.Helper()
	dir := t.TempDir()
	if strings.Contains(dir, "'") {
		t.Fatalf("temporary directory %q cannot be quoted for sh", dir)
	}
	onTerm := "exit 0"
	if survivesTerm {
		onTerm = ":"
	}
	member := filepath.Join(dir, "member")
	// The pid is written through a rename so a reader never sees a partial file.
	memberScript := "#!/bin/sh\n" +
		"exec 2>/dev/null\n" +
		"marks='" + dir + "'\n" +
		"trap 'echo TERM >> \"$marks/$1.term\"; " + onTerm + "' TERM\n" +
		"echo $$ > \"$marks/$1.pid.tmp\" && mv \"$marks/$1.pid.tmp\" \"$marks/$1.pid\"\n" +
		"while :; do sleep 0.05; done\n"
	command := filepath.Join(dir, "command")
	commandScript := "#!/bin/sh\n" +
		"'" + member + "' child &\n" +
		"exec '" + member + "' parent\n"
	for path, script := range map[string]string{member: memberScript, command: commandScript} {
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &GroupFixture{Command: command, marks: dir}
	t.Cleanup(f.kill)
	return f
}

// WaitReady waits until both processes are running with their SIGTERM
// handlers installed, and returns the process group ID of the command.
func (f *GroupFixture) WaitReady(t testing.TB) int {
	t.Helper()
	deadline := time.Now().Add(readyTimeout)
	for {
		pids, err := f.pids()
		if err == nil {
			f.group = pids["parent"]
			return f.group
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake command did not start: %v", err)
		}
		time.Sleep(pollInterval)
	}
}

// Terminated returns the members that received SIGTERM, sorted by name.
func (f *GroupFixture) Terminated(t testing.TB) []string {
	t.Helper()
	var got []string
	for _, name := range Members {
		data, err := os.ReadFile(filepath.Join(f.marks, name+".term"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(data)) != "" {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	return got
}

// WaitGone fails the test unless every process in the command's group has
// exited within timeout.
func (f *GroupFixture) WaitGone(t testing.TB, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for f.groupAlive() {
		if time.Now().After(deadline) {
			t.Fatalf("process group %d is still running", f.group)
		}
		time.Sleep(pollInterval)
	}
}

func (f *GroupFixture) pids() (map[string]int, error) {
	pids := make(map[string]int, len(Members))
	for _, name := range Members {
		data, err := os.ReadFile(filepath.Join(f.marks, name+".pid"))
		if err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, err
		}
		pids[name] = pid
	}
	return pids, nil
}

func (f *GroupFixture) groupAlive() bool {
	return f.group > 0 && !errors.Is(syscall.Kill(-f.group, 0), syscall.ESRCH)
}

// kill stops whatever is left of the fake command when a test ends early,
// including before WaitReady returned or when the code under test never put
// the command in its own process group.
func (f *GroupFixture) kill() {
	for _, name := range Members {
		data, err := os.ReadFile(filepath.Join(f.marks, name+".pid"))
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 0 {
			continue
		}
		if name == "parent" && f.group == 0 {
			f.group = pid
		}
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if f.groupAlive() {
		_ = syscall.Kill(-f.group, syscall.SIGKILL)
	}
}
