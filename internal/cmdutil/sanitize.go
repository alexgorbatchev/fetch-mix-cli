package cmdutil

import (
	"github.com/alexgorbatchev/godeps"
)

// SanitizeStderr delegates to godeps.SanitizeStderr to clean raw error outputs.
func SanitizeStderr(stderr string) string {
	return godeps.SanitizeStderr(stderr)
}
