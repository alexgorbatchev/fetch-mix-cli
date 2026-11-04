package deps

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetManagedBinDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/share")
	dir, err := GetManagedBinDir()
	if err != nil {
		t.Fatalf("GetManagedBinDir() error = %v", err)
	}
	if dir != "/custom/share/fetch-mix/bin" {
		t.Errorf("GetManagedBinDir() with XDG_DATA_HOME = %q, want '/custom/share/fetch-mix/bin'", dir)
	}

	t.Setenv("XDG_DATA_HOME", "")
	dir, err = GetManagedBinDir()
	if err != nil {
		t.Fatalf("GetManagedBinDir() error = %v", err)
	}
	if dir == "" {
		t.Fatalf("GetManagedBinDir() returned empty string")
	}
	if !strings.HasSuffix(dir, filepath.Join("fetch-mix", "bin")) {
		t.Errorf("GetManagedBinDir() = %q, want suffix 'fetch-mix/bin'", dir)
	}
}

func TestEnsureManagedBinDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	binDir, err := EnsureManagedBinDir()
	if err != nil {
		t.Fatalf("EnsureManagedBinDir() error = %v", err)
	}
	if binDir == "" {
		t.Errorf("expected non-empty binDir")
	}
}

// The pipeline must run the copy the dependency manager would choose: a copy
// elsewhere on PATH that meets the minimum version, otherwise a satisfying
// managed copy, including one installed as a package directory like yt-dlp.
func TestInitManagedPath(t *testing.T) {
	const name = "managed-path-fixture"
	tests := []struct {
		name        string
		global      string
		managed     string
		packageDir  bool
		wantVersion string
	}{
		{name: "satisfying global copy wins", global: "2.0.0", managed: "3.0.0", wantVersion: "2.0.0"},
		{name: "outdated global copy yields to managed copy", global: "0.1.0", managed: "3.0.0", wantVersion: "3.0.0"},
		{name: "outdated managed copy stays behind global copy", global: "0.1.0", managed: "0.2.0", wantVersion: "0.1.0"},
		{name: "managed copy fills in a missing global copy", managed: "3.0.0", wantVersion: "3.0.0"},
		{name: "managed package directory yields to outdated global copy", global: "0.1.0", managed: "3.0.0", packageDir: true, wantVersion: "3.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			globalDir := t.TempDir()
			t.Setenv("PATH", globalDir)
			original := RequiredDependencies
			t.Cleanup(func() { RequiredDependencies = original })
			RequiredDependencies = []Dependency{{Name: name, MinVersion: "1.0.0"}}

			if tt.global != "" {
				writeVersionScript(t, globalDir, name, tt.global)
			}
			managedDir, err := EnsureManagedBinDir()
			if err != nil {
				t.Fatal(err)
			}
			if tt.packageDir {
				managedDir = filepath.Join(managedDir, name)
				if err := os.Mkdir(managedDir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeVersionScript(t, managedDir, name, tt.managed)

			if err := InitManagedPath(context.Background()); err != nil {
				t.Fatalf("InitManagedPath() error = %v", err)
			}
			out, err := exec.Command(name).Output()
			if err != nil {
				t.Fatalf("running %s from PATH %q: %v", name, os.Getenv("PATH"), err)
			}
			if got := strings.TrimSpace(string(out)); got != tt.wantVersion {
				t.Fatalf("%s resolved to version %s, want %s", name, got, tt.wantVersion)
			}
		})
	}
}

func TestInitManagedPathCanceled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := InitManagedPath(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("InitManagedPath() error = %v, want context.Canceled", err)
	}
}

func writeVersionScript(t *testing.T, dir, name, version string) {
	t.Helper()
	script := "#!/bin/sh\nprintf '%s\\n' '" + version + "'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
