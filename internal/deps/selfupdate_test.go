package deps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
