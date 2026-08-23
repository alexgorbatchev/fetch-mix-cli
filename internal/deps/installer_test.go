package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestInstallFFmpeg_OSBranches(t *testing.T) {
	ctx := context.Background()
	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	oldLookPath := lookPathFunc
	defer func() { lookPathFunc = oldLookPath }()

	// Test each linux package manager
	for _, pm := range []string{"apt-get", "pacman", "dnf"} {
		lookPathFunc = func(file string) (string, error) {
			if file == pm {
				return "/usr/bin/" + pm, nil
			}
			return "", errors.New("not found")
		}
		_ = installFFmpegForOS(ctx, "linux", mockRunner)
	}

	// Test each windows package manager
	for _, pm := range []string{"winget", "choco"} {
		lookPathFunc = func(file string) (string, error) {
			if file == pm {
				return "/usr/bin/" + pm, nil
			}
			return "", errors.New("not found")
		}
		_ = installFFmpegForOS(ctx, "windows", mockRunner)
	}

	// Test darwin
	lookPathFunc = func(file string) (string, error) {
		if file == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", errors.New("not found")
	}
	_ = installFFmpegForOS(ctx, "darwin", mockRunner)

	// Test unsupported or none found
	lookPathFunc = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	_ = installFFmpegForOS(ctx, "darwin", mockRunner)
	_ = installFFmpegForOS(ctx, "linux", mockRunner)
	_ = installFFmpegForOS(ctx, "windows", mockRunner)
	_ = installFFmpegForOS(ctx, "freebsd", mockRunner)
}

func TestEnsureManagedBinDir(t *testing.T) {
	dir, err := EnsureManagedBinDir()
	if err != nil {
		t.Fatalf("EnsureManagedBinDir() error = %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("EnsureManagedBinDir() did not create dir: %v", err)
	}

	// MkdirAll error
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file_as_share")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	t.Setenv("XDG_DATA_HOME", filePath)
	if _, err := EnsureManagedBinDir(); err == nil {
		t.Errorf("expected error when MkdirAll fails")
	}

	// Home dir error
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	_, _ = GetManagedBinDir()
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

func TestDownloadAndExtractAsset_Errors(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// 1. 404 error
	ts404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts404.Close()

	if err := downloadAndExtractAsset(ctx, ts404.URL, "bin", tempDir, false); err == nil {
		t.Errorf("expected error for 404 asset download")
	}

	// 2. Corrupted tar.gz
	tsBadTar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a tar.gz"))
	}))
	defer tsBadTar.Close()

	if err := downloadAndExtractAsset(ctx, tsBadTar.URL, "bin", tempDir, false); err == nil {
		t.Errorf("expected error for corrupted tar.gz")
	}

	// 3. Corrupted zip
	if err := downloadAndExtractAsset(ctx, tsBadTar.URL, "bin.exe", tempDir, true); err == nil {
		t.Errorf("expected error for corrupted zip")
	}

	// 4. Direct binary 404
	if err := downloadDirectBinary(ctx, ts404.URL, "bin", tempDir); err == nil {
		t.Errorf("expected error for 404 direct download")
	}

	// 5. Network errors
	_ = downloadDirectBinary(ctx, "http://invalid-non-existent-domain-12345.org", "bin", tempDir)
	_ = downloadAndExtractAsset(ctx, "http://invalid-non-existent-domain-12345.org", "bin", tempDir, false)

	// 6. Existing directory as target binary path (triggers rename error)
	dirAsBinary := filepath.Join(tempDir, "bin_is_dir")
	_ = os.MkdirAll(filepath.Join(dirAsBinary, "fetch-track"), 0755)
	tsOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(createTestTarGz(t, "fetch-track", "content"))
	}))
	defer tsOk.Close()

	_ = downloadAndExtractAsset(ctx, tsOk.URL, "fetch-track", dirAsBinary, false)

	// 7. Tar without matching binary name
	tarNoMatch := createTestTarGz(t, "unmatched_binary", "data")
	tsNoMatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(tarNoMatch)
	}))
	defer tsNoMatch.Close()

	if err := downloadAndExtractAsset(ctx, tsNoMatch.URL, "wanted_binary", tempDir, false); err == nil {
		t.Errorf("expected error when tar does not contain wanted binary")
	}

	// 8. Zip without matching binary name
	zipNoMatch := createTestZip(t, "unmatched_binary.exe", "data")
	tsZipNoMatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipNoMatch)
	}))
	defer tsZipNoMatch.Close()

	if err := downloadAndExtractAsset(ctx, tsZipNoMatch.URL, "wanted_binary.exe", tempDir, true); err == nil {
		t.Errorf("expected error when zip does not contain wanted binary")
	}
}

func TestResolveLatestTag_MoreErrors(t *testing.T) {
	ctx := context.Background()

	// 1. 200 OK instead of redirect
	ts200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts200.Close()

	if _, err := ResolveLatestTagWithBaseURL(ctx, ts200.URL, "owner", "repo"); err == nil {
		t.Errorf("expected error for 200 response without redirect")
	}

	// 2. Empty trailing tag in location
	tsEmptyTag := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)
	}))
	defer tsEmptyTag.Close()
	if _, err := ResolveLatestTagWithBaseURL(ctx, tsEmptyTag.URL, "owner", "repo"); err == nil {
		t.Errorf("expected error for empty tag location")
	}

	// 4. HEAD fails with 405, GET fallback succeeds
	tsHeadFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, "/owner/repo/releases/tag/v1.0.0", http.StatusFound)
	}))
	defer tsHeadFail.Close()
	tag, err := ResolveLatestTagWithBaseURL(ctx, tsHeadFail.URL, "owner", "repo")
	if err != nil || tag != "v1.0.0" {
		t.Errorf("expected tag v1.0.0 from GET fallback, got %q, %v", tag, err)
	}
}

func TestInstallYtDlpForPlatform(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	var requestedPaths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Write([]byte("#!/bin/sh\n"))
	}))
	defer ts.Close()

	oldBase := githubBaseURL
	githubBaseURL = ts.URL
	defer func() { githubBaseURL = oldBase }()

	tests := []struct {
		goos     string
		goarch   string
		wantPath string
	}{
		{"darwin", "arm64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"},
		{"darwin", "amd64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"},
		{"windows", "amd64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"},
		{"linux", "arm64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64"},
		{"linux", "amd64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"},
		{"freebsd", "amd64", "/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"},
	}

	for _, tt := range tests {
		requestedPaths = nil
		err := installYtDlpForPlatform(ctx, tt.goos, tt.goarch, tempDir)
		if err != nil {
			t.Fatalf("installYtDlpForPlatform(%s, %s) error: %v", tt.goos, tt.goarch, err)
		}
		if len(requestedPaths) == 0 || requestedPaths[len(requestedPaths)-1] != tt.wantPath {
			t.Errorf("installYtDlpForPlatform(%s, %s) requested %v, want %s", tt.goos, tt.goarch, requestedPaths, tt.wantPath)
		}
	}
}

func TestUpdateYtDlp(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// 1. Success via runner
	okRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("yt-dlp is up to date"), nil
	}
	if err := UpdateYtDlp(ctx, okRunner, tempDir); err != nil {
		t.Errorf("UpdateYtDlp failed on success runner: %v", err)
	}

	// 2. Runner returns error or "error" in output -> triggers fallback to download
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#!/bin/sh\n"))
	}))
	defer ts.Close()

	oldBase := githubBaseURL
	githubBaseURL = ts.URL
	defer func() { githubBaseURL = oldBase }()

	errRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ERROR: update failed"), errors.New("exit 1")
	}
	_ = UpdateYtDlp(ctx, errRunner, tempDir)
}

func TestDownloadAndExtractGoBinary_Mock(t *testing.T) {
	binContent := "#!/bin/sh\necho 1.0.0\n"
	tarGzData := createTestTarGz(t, "fetch-track", binContent)
	zipData := createTestZip(t, "fetch-track.exe", "windows content")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			http.Redirect(w, r, "/owner/repo/releases/tag/v1.0.0", http.StatusFound)
		case strings.Contains(r.URL.Path, ".zip"):
			w.Header().Set("Content-Type", "application/zip")
			w.Write(zipData)
		case strings.Contains(r.URL.Path, "/releases/download/v1.0.0/"):
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(tarGzData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	oldBase := githubBaseURL
	githubBaseURL = ts.URL
	defer func() { githubBaseURL = oldBase }()

	tempDir := t.TempDir()
	ctx := context.Background()
	err := DownloadAndExtractGoBinary(ctx, "owner", "repo", "fetch-track", tempDir)
	if err != nil {
		t.Fatalf("DownloadAndExtractGoBinary failed: %v", err)
	}

	dest := filepath.Join(tempDir, "fetch-track")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("extracted binary missing: %v", err)
	}

	// Test windows OS branch
	err = downloadAndExtractGoBinaryForOS(ctx, "windows", "amd64", "owner", "repo", "fetch-track", tempDir)
	if err != nil {
		t.Fatalf("downloadAndExtractGoBinaryForOS windows failed: %v", err)
	}

	// Test yt-dlp windows OS branch
	_ = installYtDlpForOS(ctx, "windows", tempDir)
}

func TestInstallAndVerify_Integration(t *testing.T) {
	binContent := "#!/bin/sh\necho 1.0.0\n"
	tarGzData := createTestTarGz(t, "fetch-track", binContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			http.Redirect(w, r, "/owner/repo/releases/tag/v1.0.0", http.StatusFound)
		case strings.Contains(r.URL.Path, "/releases/download/"):
			if strings.HasSuffix(r.URL.Path, ".tar.gz") {
				w.Header().Set("Content-Type", "application/gzip")
				w.Write(tarGzData)
			} else {
				w.Write([]byte(binContent))
			}
		default:
			w.Write([]byte(binContent))
		}
	}))
	defer ts.Close()

	oldBase := githubBaseURL
	oldRunner := packageManagerRunner
	githubBaseURL = ts.URL
	packageManagerRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}
	defer func() {
		githubBaseURL = oldBase
		packageManagerRunner = oldRunner
	}()

	ctx := context.Background()
	_ = InitManagedPath()

	_ = InstallDependency(ctx, "fetch-track")
	_ = InstallDependency(ctx, "yt-dlp")
	_ = InstallDependency(ctx, "ffmpeg")
	if err := InstallDependency(ctx, "unknown"); err == nil {
		t.Errorf("expected error for unknown dep")
	}

	_ = UpdateDependency(ctx, "fetch-track")
	_ = UpdateDependency(ctx, "yt-dlp")
	_ = UpdateDependency(ctx, "ffmpeg")
	if err := UpdateDependency(ctx, "unknown"); err == nil {
		t.Errorf("expected error for unknown dep update")
	}

	_ = InstallFFmpeg(ctx, packageManagerRunner)

	_, _ = InstallMissingDependencies(ctx)
	_, _ = UpdateAllDependencies(ctx)
}

func TestGetManagedBinDir_EmptyHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	_, err := GetManagedBinDir()
	if err == nil {
		t.Errorf("expected error when both XDG_DATA_HOME and HOME are empty")
	}
}
