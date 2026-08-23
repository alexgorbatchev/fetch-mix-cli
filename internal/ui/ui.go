package ui

import (
	"os"
	"strings"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"golang.org/x/term"
)

var (
	isTerminalFunc = term.IsTerminal
	getSizeFunc    = func(fd int) (int, int, error) {
		return term.GetSize(fd)
	}
)

// Separator returns a repeated character string.
// In non-agent mode (agent=0) with an active TTY, it expands dynamically to terminal width (clamped to 120 max).
// In agent mode (agent=1) or non-TTY mode, it falls back to defaultWidth.
func Separator(char string, defaultWidth int) string {
	if defaultWidth <= 0 {
		defaultWidth = 50
	}

	if deps.IsAgentMode() {
		return strings.Repeat(char, defaultWidth)
	}

	fd := int(os.Stdout.Fd())
	if isTerminalFunc(fd) {
		width, _, err := getSizeFunc(fd)
		if err == nil && width >= 20 {
			if width > 120 {
				width = 120
			}
			return strings.Repeat(char, width)
		}
	}

	return strings.Repeat(char, defaultWidth)
}
