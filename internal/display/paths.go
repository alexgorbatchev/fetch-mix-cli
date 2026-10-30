package display

import (
	"os"
	"path/filepath"
	"strings"
)

// Path folds the home directory for display, preserving actual filesystem paths.
func Path(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	home = filepath.Clean(home)
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
