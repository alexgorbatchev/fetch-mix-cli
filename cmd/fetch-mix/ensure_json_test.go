package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/godeps"
)

// JSON callers read only the machine result from stdout, so auto-installation
// must report an installation that still leaves a dependency outdated, and must
// keep its messages off stdout.
func TestEnsureDependencies_JSONAutoInstallVerifiesInstallation(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		wantErr   bool
	}{
		{name: "installation provides a supported version", installed: "1.5.0"},
		{name: "installation leaves the dependency outdated", installed: "1.0.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("HOME", t.TempDir())
			bin := t.TempDir()
			t.Setenv("PATH", bin)
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("AGENT", "1")
			writeVersionTool(t, bin, "1.0.0")
			original := deps.RequiredDependencies
			deps.RequiredDependencies = []deps.Dependency{{
				Name:       "fetch-mix-test-tool",
				MinVersion: "1.4.0",
				Installer: godeps.InstallerFunc(func(context.Context, string) error {
					writeVersionTool(t, bin, tt.installed)
					return nil
				}),
			}}
			jsonOutput, autoInstall = true, true
			t.Cleanup(func() {
				deps.RequiredDependencies = original
				jsonOutput, autoInstall = false, false
			})

			stdout := captureStdout(t)
			err := ensureDependencies(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("ensureDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
			if out := stdout(); out != "" {
				t.Errorf("stdout = %q, want no output in JSON mode", out)
			}
		})
	}
}

func writeVersionTool(t *testing.T, dir, version string) {
	t.Helper()
	script := "#!/bin/sh\necho " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fetch-mix-test-tool"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

// captureStdout redirects os.Stdout and returns a function that restores it and
// returns everything written.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = original })
	return func() string {
		os.Stdout = original
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}
