package cmdutil

import (
	"context"
	"testing"
	"time"
)

func TestNewCommand_Execution(t *testing.T) {
	ctx := context.Background()
	cmd := NewCommand(ctx, "echo", "hello")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("NewCommand echo failed: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestNewCommand_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := NewCommand(ctx, "sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() failed: %v", err)
	}

	// Trigger cancellation
	time.Sleep(20 * time.Millisecond)
	cancel()

	err := cmd.Wait()
	if err == nil {
		t.Errorf("expected error from canceled process, got nil")
	}
}
