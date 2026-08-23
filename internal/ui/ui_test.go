package ui

import (
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
	if len(sepAgent) != 50 {
		t.Errorf("In agent=1 mode, expected separator length 50, got %d", len(sepAgent))
	}

	os.Unsetenv("AGENT")
}
