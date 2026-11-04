package deps

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

// runIsolated keeps the dependency manager away from the real managed
// directory and cache, so tests never run or replace installed tools.
func runIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "fetch-mix-deps-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	for _, key := range []string{"HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		if err := os.Setenv(key, dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return m.Run()
}
