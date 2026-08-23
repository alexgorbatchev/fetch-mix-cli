package deps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeSelfToPath(t *testing.T) {
	tempDir := t.TempDir()
	targetExe := filepath.Join(tempDir, "fetch-mix")

	// Write old mock binary
	oldContent := "#!/bin/sh\necho old-version\n"
	if err := os.WriteFile(targetExe, []byte(oldContent), 0755); err != nil {
		t.Fatalf("failed to write initial binary: %v", err)
	}

	newContent := "#!/bin/sh\necho 2.0.0\n"
	tarGzData := createTestTarGz(t, "fetch-mix", newContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/alexgorbatchev/fetch-mix-cli/releases/latest":
			http.Redirect(w, r, "/alexgorbatchev/fetch-mix-cli/releases/tag/v2.0.0", http.StatusFound)
		case "/alexgorbatchev/fetch-mix-cli/releases/download/v2.0.0/fetch-mix_2.0.0_darwin_arm64.tar.gz",
			"/alexgorbatchev/fetch-mix-cli/releases/download/v2.0.0/fetch-mix_2.0.0_darwin_amd64.tar.gz",
			"/alexgorbatchev/fetch-mix-cli/releases/download/v2.0.0/fetch-mix_2.0.0_linux_amd64.tar.gz",
			"/alexgorbatchev/fetch-mix-cli/releases/download/v2.0.0/fetch-mix_2.0.0_linux_arm64.tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarGzData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := context.Background()

	// 1. When current version is older (1.0.0 < 2.0.0), it should upgrade
	updated, newVer, err := UpgradeSelfWithBaseURL(ctx, ts.URL, "1.0.0", targetExe)
	if err != nil {
		t.Fatalf("UpgradeSelfWithBaseURL() error = %v", err)
	}
	if !updated {
		t.Errorf("expected updated = true, got false")
	}
	if newVer != "2.0.0" {
		t.Errorf("expected newVer = '2.0.0', got %q", newVer)
	}

	// Verify target executable was replaced with new content
	data, err := os.ReadFile(targetExe)
	if err != nil {
		t.Fatalf("failed to read upgraded binary: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("binary content = %q, want %q", string(data), newContent)
	}

	// 2. When current version is already 2.0.0, it should report up to date
	updated, sameVer, err := UpgradeSelfWithBaseURL(ctx, ts.URL, "2.0.0", targetExe)
	if err != nil {
		t.Fatalf("UpgradeSelfWithBaseURL() error = %v", err)
	}
	if updated {
		t.Errorf("expected updated = false when already on latest version, got true")
	}
	if sameVer != "2.0.0" {
		t.Errorf("expected sameVer = '2.0.0', got %q", sameVer)
	}
}

func TestUpgradeSelf_DevVersion(t *testing.T) {
	tempDir := t.TempDir()
	targetExe := filepath.Join(tempDir, "fetch-mix")
	_ = os.WriteFile(targetExe, []byte("#!/bin/sh\n"), 0755)

	binContent := "#!/bin/sh\necho 1.0.0\n"
	tarGzData := createTestTarGz(t, "fetch-mix", binContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			http.Redirect(w, r, "/alexgorbatchev/fetch-mix-cli/releases/tag/v1.0.0", http.StatusFound)
		case strings.Contains(r.URL.Path, "/releases/download/"):
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarGzData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	updated, ver, err := UpgradeSelfWithBaseURL(ctx, ts.URL, "dev", filepath.Join(tempDir, "custom-fetch-mix"))
	if err != nil || !updated || ver != "1.0.0" {
		t.Errorf("expected updated=true, ver=1.0.0, got %v, %q, %v", updated, ver, err)
	}

	// Empty current version
	updated, ver, err = UpgradeSelfWithBaseURL(ctx, ts.URL, "", filepath.Join(tempDir, "custom-fetch-mix-2"))
	if err != nil || !updated || ver != "1.0.0" {
		t.Errorf("expected updated=true for empty version, got %v, %q, %v", updated, ver, err)
	}

	// Executable error in UpgradeSelf
	oldExeFunc := executablePathFunc
	defer func() { executablePathFunc = oldExeFunc }()
	executablePathFunc = func() (string, error) {
		return "", errors.New("cannot locate executable")
	}
	if _, _, err := UpgradeSelf(ctx, "1.0.0"); err == nil {
		t.Errorf("expected error when locating executable fails")
	}

	executablePathFunc = func() (string, error) {
		return "/nonexistent/invalid/symlink", nil
	}
	if _, _, err := UpgradeSelf(ctx, "1.0.0"); err == nil {
		t.Errorf("expected error when symlink evaluation fails")
	}
}

func TestUpgradeSelf_Call(t *testing.T) {
	tempDir := t.TempDir()
	_ = tempDir
	binContent := "#!/bin/sh\necho 2.0.0\n"
	tarGzData := createTestTarGz(t, "fetch-mix", binContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			http.Redirect(w, r, "/alexgorbatchev/fetch-mix-cli/releases/tag/v2.0.0", http.StatusFound)
		case strings.Contains(r.URL.Path, "/releases/download/"):
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarGzData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	oldBase := selfUpdateBaseURL
	selfUpdateBaseURL = ts.URL
	defer func() { selfUpdateBaseURL = oldBase }()

	ctx := context.Background()
	_, _, _ = UpgradeSelf(ctx, "v1.0.0")
	_, _, _ = UpgradeSelf(ctx, "0.1.0")
}
