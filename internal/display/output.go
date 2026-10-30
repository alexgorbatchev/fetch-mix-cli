package display

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Text folds home paths embedded in user-facing diagnostics.
func Text(text string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return text
	}
	var out strings.Builder
	for {
		at := strings.Index(text, home)
		if at < 0 {
			out.WriteString(text)
			break
		}
		end := at + len(home)
		before := at == 0 || strings.ContainsRune(" \t\n\r\"'=:([{", rune(text[at-1]))
		after := end == len(text) || strings.ContainsRune("/\\ \t\n\r\"':;,)]}", rune(text[end]))
		if before && after {
			out.WriteString(text[:at])
			out.WriteString(Path(home))
			text = text[end:]
		} else {
			out.WriteString(text[:end])
			text = text[end:]
		}
	}
	return out.String()
}

// Printf writes a formatted terminal message with display paths.
func Printf(format string, args ...any) (int, error) {
	return fmt.Fprint(os.Stdout, Text(fmt.Sprintf(format, args...)))
}

// Println writes a terminal line with display paths.
func Println(args ...any) (int, error) { return fmt.Fprint(os.Stdout, Text(fmt.Sprintln(args...))) }

// Fprintf writes a diagnostic to the supplied terminal stream.
func Fprintf(w io.Writer, format string, args ...any) (int, error) {
	return fmt.Fprint(w, Text(fmt.Sprintf(format, args...)))
}
