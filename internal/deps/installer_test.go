package deps

import (
	"context"
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

func TestInitManagedPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	err := InitManagedPath()
	if err != nil {
		t.Fatalf("InitManagedPath() error = %v", err)
	}
}

func TestUpgradeSelf_DevVersion(t *testing.T) {
	ctx := context.Background()
	_, _, _ = UpgradeSelf(ctx, "dev")
}
