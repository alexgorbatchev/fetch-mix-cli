package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetManagedBinDir(t *testing.T) {
	dir, err := GetManagedBinDir()
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

func TestResolveLatestTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/owner/repo/releases/latest" {
			http.Redirect(w, r, "/owner/repo/releases/tag/v1.4.2", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	ctx := context.Background()
	tag, err := ResolveLatestTagWithBaseURL(ctx, ts.URL, "owner", "repo")
	if err != nil {
		t.Fatalf("ResolveLatestTagWithBaseURL() error = %v", err)
	}
	if tag != "v1.4.2" {
		t.Errorf("ResolveLatestTagWithBaseURL() = %q, want 'v1.4.2'", tag)
	}
}

func createTestTarGz(t *testing.T, binName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	data := []byte(content)
	hdr := &tar.Header{
		Name: binName,
		Mode: 0755,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createTestZip(t *testing.T, binName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	data := []byte(content)
	f, err := zw.Create(binName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadAndExtractBinary_TarGz(t *testing.T) {
	binContent := "#!/bin/sh\necho 1.4.2\n"
	tarGzData := createTestTarGz(t, "fetch-track", binContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(tarGzData)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	err := downloadAndExtractAsset(ctx, ts.URL, "fetch-track", tempDir, false)
	if err != nil {
		t.Fatalf("downloadAndExtractAsset() error = %v", err)
	}

	destBinary := filepath.Join(tempDir, "fetch-track")
	data, err := os.ReadFile(destBinary)
	if err != nil {
		t.Fatalf("failed to read extracted binary: %v", err)
	}
	if string(data) != binContent {
		t.Errorf("binary content = %q, want %q", string(data), binContent)
	}

	// Verify permissions on Unix
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(destBinary)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0111 == 0 {
			t.Errorf("expected binary to be executable, mode = %v", fi.Mode())
		}
	}
}

func TestDownloadAndExtractBinary_Zip(t *testing.T) {
	binContent := "windows binary content"
	zipData := createTestZip(t, "fetch-track.exe", binContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	err := downloadAndExtractAsset(ctx, ts.URL, "fetch-track.exe", tempDir, true)
	if err != nil {
		t.Fatalf("downloadAndExtractAsset() error = %v", err)
	}

	destBinary := filepath.Join(tempDir, "fetch-track.exe")
	data, err := os.ReadFile(destBinary)
	if err != nil {
		t.Fatalf("failed to read extracted binary: %v", err)
	}
	if string(data) != binContent {
		t.Errorf("binary content = %q, want %q", string(data), binContent)
	}
}

func TestInstallDirectBinary(t *testing.T) {
	binContent := "#!/bin/sh\necho 2026.08.19\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(binContent))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	err := downloadDirectBinary(ctx, ts.URL, "yt-dlp", tempDir)
	if err != nil {
		t.Fatalf("downloadDirectBinary() error = %v", err)
	}

	destBinary := filepath.Join(tempDir, "yt-dlp")
	data, err := os.ReadFile(destBinary)
	if err != nil {
		t.Fatalf("failed to read downloaded binary: %v", err)
	}
	if string(data) != binContent {
		t.Errorf("binary content = %q, want %q", string(data), binContent)
	}
}

func TestResolveLatestTag_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("404_not_found", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer ts.Close()

		_, err := ResolveLatestTagWithBaseURL(ctx, ts.URL, "owner", "repo")
		if err == nil {
			t.Errorf("expected error for 404 response, got nil")
		}
	})

	t.Run("missing_location_header", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusFound)
		}))
		defer ts.Close()

		_, err := ResolveLatestTagWithBaseURL(ctx, ts.URL, "owner", "repo")
		if err == nil {
			t.Errorf("expected error for missing Location header, got nil")
		}
	})
}

func TestInitManagedPath(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	if err := InitManagedPath(); err != nil {
		t.Fatalf("InitManagedPath() error = %v", err)
	}

	binDir, err := GetManagedBinDir()
	if err != nil {
		t.Fatalf("GetManagedBinDir() error = %v", err)
	}

	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, binDir) {
		t.Errorf("PATH does not contain managed bin directory %q: %s", binDir, currentPath)
	}

	// Calling again should be idempotent
	if err := InitManagedPath(); err != nil {
		t.Fatalf("Second InitManagedPath() error = %v", err)
	}
}

func TestInstallDependency_Unknown(t *testing.T) {
	ctx := context.Background()
	err := InstallDependency(ctx, "nonexistent-dep")
	if err == nil {
		t.Errorf("expected error for unknown dependency, got nil")
	}
}
