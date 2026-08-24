package ui

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSeparator(t *testing.T) {
	os.Unsetenv("AGENT")

	sep := Separator("=", 50)
	if len(sep) < 50 {
		t.Errorf("Separator len = %d, expected at least 50", len(sep))
	}
	if !strings.HasPrefix(sep, "===") {
		t.Errorf("Separator string does not start with '='")
	}

	os.Setenv("AGENT", "1")
	sepAgent := Separator("=", 50)
	if sepAgent != "" {
		t.Errorf("In agent=1 mode, expected empty separator, got %q", sepAgent)
	}
	os.Unsetenv("AGENT")

	sepDefault := Separator("-", 0)
	if len(sepDefault) != 50 {
		t.Errorf("Separator with default 0 width expected 50, got %d", len(sepDefault))
	}
}

func TestSeparator_TerminalBranches(t *testing.T) {
	os.Unsetenv("AGENT")
	origIsTerm := isTerminalFunc
	origGetSize := getSizeFunc
	defer func() {
		isTerminalFunc = origIsTerm
		getSizeFunc = origGetSize
	}()

	isTerminalFunc = func(fd int) bool { return true }

	// Test standard terminal width
	getSizeFunc = func(fd int) (int, int, error) { return 80, 24, nil }
	if s := Separator("=", 50); len(s) != 80 {
		t.Errorf("expected width 80, got %d", len(s))
	}

	// Test wide terminal (clamped to 120)
	getSizeFunc = func(fd int) (int, int, error) { return 200, 24, nil }
	if s := Separator("=", 50); len(s) != 120 {
		t.Errorf("expected clamped width 120, got %d", len(s))
	}

	// Test narrow terminal (< 20 fallback)
	getSizeFunc = func(fd int) (int, int, error) { return 10, 24, nil }
	if s := Separator("=", 50); len(s) != 50 {
		t.Errorf("expected fallback width 50, got %d", len(s))
	}

	// Test error from getSizeFunc
	getSizeFunc = func(fd int) (int, int, error) { return 0, 0, errors.New("err") }
	if s := Separator("=", 50); len(s) != 50 {
		t.Errorf("expected fallback width 50 on error, got %d", len(s))
	}
}
