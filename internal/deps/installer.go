package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

// GetManagedBinDir returns the path to the fetch-mix managed binary directory following XDG Base Directory standards.
func GetManagedBinDir() (string, error) {
	if xdgData := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgData != "" {
		return filepath.Join(xdgData, "fetch-mix", "bin"), nil
	}

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "fetch-mix", "bin"), nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "fetch-mix", "bin"), nil
}

// EnsureManagedBinDir creates the managed binary directory if it does not exist and returns its path.
func EnsureManagedBinDir() (string, error) {
	binDir, err := GetManagedBinDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("creating managed bin directory %q: %w", binDir, err)
	}

	return binDir, nil
}

// InitManagedPath ensures the managed bin directory is prepended to the process PATH environment variable.
func InitManagedPath() error {
	binDir, err := EnsureManagedBinDir()
	if err != nil {
		return err
	}

	currentPath := os.Getenv("PATH")
	pathList := filepath.SplitList(currentPath)
	for _, p := range pathList {
		if filepath.Clean(p) == filepath.Clean(binDir) {
			return nil
		}
	}

	newPath := binDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}

var (
	githubBaseURL        = "https://github.com"
	packageManagerRunner = DefaultRunner
	lookPathFunc         = exec.LookPath
)

// ResolveLatestTag queries the latest release tag for a GitHub repository without using the GitHub API.
func ResolveLatestTag(ctx context.Context, owner, repo string) (string, error) {
	return ResolveLatestTagWithBaseURL(ctx, githubBaseURL, owner, repo)
}

// ResolveLatestTagWithBaseURL queries the latest release tag for a repository from a custom base URL.
func ResolveLatestTagWithBaseURL(ctx context.Context, baseURL, owner, repo string) (string, error) {
	targetURL := fmt.Sprintf("%s/%s/%s/releases/latest", strings.TrimRight(baseURL, "/"), owner, repo)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop at the first redirect to read the Location header
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request for latest release: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil || (resp != nil && (resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusBadRequest)) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		// Fall back to GET if HEAD method fails or is rejected
		reqGet, getErr := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if getErr == nil {
			resp, err = client.Do(reqGet)
		}
		if err != nil {
			return "", fmt.Errorf("fetching latest release redirect from %s: %w", targetURL, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, targetURL)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no Location header returned from %s", targetURL)
	}

	parts := strings.Split(strings.TrimRight(location, "/"), "/")
	tag := parts[len(parts)-1]
	if tag == "" {
		return "", fmt.Errorf("failed to extract tag from Location %q", location)
	}

	return tag, nil
}

// DownloadAndExtractGoBinary downloads and extracts a Go binary from a GitHub release without using GitHub API.
func DownloadAndExtractGoBinary(ctx context.Context, owner, repo, binName, targetDir string) error {
	return downloadAndExtractGoBinaryForOS(ctx, runtime.GOOS, runtime.GOARCH, owner, repo, binName, targetDir)
}

func downloadAndExtractGoBinaryForOS(ctx context.Context, osName, archName, owner, repo, binName, targetDir string) error {
	tag, err := ResolveLatestTag(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("resolving latest release for %s/%s: %w", owner, repo, err)
	}

	cleanVer := strings.TrimPrefix(tag, "v")

	isZip := (osName == "windows")
	ext := "tar.gz"
	execName := binName
	if isZip {
		ext = "zip"
		execName = binName + ".exe"
	}

	assetName := fmt.Sprintf("%s_%s_%s_%s.%s", binName, cleanVer, osName, archName, ext)
	assetURL := fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", strings.TrimRight(githubBaseURL, "/"), owner, repo, tag, assetName)

	return downloadAndExtractAsset(ctx, assetURL, execName, targetDir, isZip)
}

func downloadAndExtractAsset(ctx context.Context, assetURL, binName, targetDir string, isZip bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return fmt.Errorf("creating asset download request: %w", err)
	}

	client := &http.Client{
		Timeout: 3 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading asset from %s: %w", assetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download failed with HTTP %d from %s", resp.StatusCode, assetURL)
	}

	destPath := filepath.Join(targetDir, binName)
	tmpPath := destPath + ".tmp"
	_ = os.Remove(tmpPath)

	if isZip {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading zip archive: %w", err)
		}

		zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		if err != nil {
			return fmt.Errorf("parsing zip archive: %w", err)
		}

		var matchedFile *zip.File
		for _, f := range zipReader.File {
			base := filepath.Base(f.Name)
			if base == binName || strings.EqualFold(base, binName) {
				matchedFile = f
				break
			}
		}

		if matchedFile == nil {
			return fmt.Errorf("executable %q not found in zip archive", binName)
		}

		rc, err := matchedFile.Open()
		if err != nil {
			return fmt.Errorf("opening zip file entry: %w", err)
		}
		defer rc.Close()

		outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("creating temporary binary %q: %w", tmpPath, err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			_ = outFile.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("writing extracted binary: %w", err)
		}
		_ = outFile.Close()
	} else {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("initializing gzip reader: %w", err)
		}
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)
		found := false

		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("reading tar archive: %w", err)
			}

			base := filepath.Base(hdr.Name)
			if (hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA) && (base == binName || hdr.Name == binName) {
				outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return fmt.Errorf("creating temporary binary %q: %w", tmpPath, err)
				}

				if _, err := io.Copy(outFile, tarReader); err != nil {
					_ = outFile.Close()
					_ = os.Remove(tmpPath)
					return fmt.Errorf("writing extracted binary: %w", err)
				}
				_ = outFile.Close()
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("executable %q not found in tar archive", binName)
		}
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting executable permissions: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("moving binary into place at %q: %w", destPath, err)
	}

	return nil
}

func downloadDirectBinary(ctx context.Context, downloadURL, binName, targetDir string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("creating binary download request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading binary from %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binary download failed with HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	destPath := filepath.Join(targetDir, binName)
	tmpPath := destPath + ".tmp"
	_ = os.Remove(tmpPath)

	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("creating temporary binary %q: %w", tmpPath, err)
	}

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		_ = outFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing downloaded binary: %w", err)
	}
	_ = outFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting executable permissions: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("moving binary to %q: %w", destPath, err)
	}

	return nil
}

// InstallYtDlp downloads the standalone yt-dlp binary into targetDir.
func InstallYtDlp(ctx context.Context, targetDir string) error {
	return installYtDlpForOS(ctx, runtime.GOOS, targetDir)
}

func installYtDlpForOS(ctx context.Context, goos, targetDir string) error {
	binName := "yt-dlp"
	downloadURL := fmt.Sprintf("%s/yt-dlp/yt-dlp/releases/latest/download/yt-dlp", strings.TrimRight(githubBaseURL, "/"))
	if goos == "windows" {
		binName = "yt-dlp.exe"
		downloadURL = fmt.Sprintf("%s/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe", strings.TrimRight(githubBaseURL, "/"))
	}

	return downloadDirectBinary(ctx, downloadURL, binName, targetDir)
}

// UpdateYtDlp attempts native yt-dlp -U update first, falling back to direct download.
func UpdateYtDlp(ctx context.Context, runner CommandRunner, targetDir string) error {
	// Try native self-update first if available in PATH
	if runner == nil {
		runner = DefaultRunner
	}
	out, err := runner(ctx, "yt-dlp", "-U")
	if err == nil && !strings.Contains(strings.ToLower(string(out)), "error") {
		return nil
	}

	// Fallback to downloading direct binary into managed bin directory
	return InstallYtDlp(ctx, targetDir)
}

// InstallFFmpeg attempts to install ffmpeg using the host system package manager.
func InstallFFmpeg(ctx context.Context, runner CommandRunner) error {
	return installFFmpegForOS(ctx, runtime.GOOS, runner)
}

func installFFmpegForOS(ctx context.Context, goos string, runner CommandRunner) error {
	if runner == nil {
		runner = DefaultRunner
	}

	switch goos {
	case "darwin":
		if _, err := lookPathFunc("brew"); err == nil {
			_, err := runner(ctx, "brew", "install", "ffmpeg")
			return err
		}
	case "linux":
		if _, err := lookPathFunc("apt-get"); err == nil {
			_, err := runner(ctx, "apt-get", "install", "-y", "ffmpeg")
			return err
		}
		if _, err := lookPathFunc("pacman"); err == nil {
			_, err := runner(ctx, "pacman", "-S", "--noconfirm", "ffmpeg")
			return err
		}
		if _, err := lookPathFunc("dnf"); err == nil {
			_, err := runner(ctx, "dnf", "install", "-y", "ffmpeg")
			return err
		}
	case "windows":
		if _, err := lookPathFunc("winget"); err == nil {
			_, err := runner(ctx, "winget", "install", "--id", "Gyan.FFmpeg", "-e", "--silent")
			return err
		}
		if _, err := lookPathFunc("choco"); err == nil {
			_, err := runner(ctx, "choco", "install", "ffmpeg", "-y")
			return err
		}
	}

	return fmt.Errorf("could not auto-install ffmpeg: no supported package manager found. Please install ffmpeg manually: https://ffmpeg.org/download.html")
}

// InstallDependency downloads or installs a single dependency into the managed bin directory.
func InstallDependency(ctx context.Context, depName string) error {
	binDir, err := EnsureManagedBinDir()
	if err != nil {
		return err
	}

	defer func() {
		if c, cErr := cache.New(); cErr == nil {
			_ = c.Delete(fmt.Sprintf("deps_%s.json", depName))
		}
	}()

	switch depName {
	case "fetch-track":
		return DownloadAndExtractGoBinary(ctx, "alexgorbatchev", "fetch-track-cli", "fetch-track", binDir)
	case "yt-dlp":
		return InstallYtDlp(ctx, binDir)
	case "ffmpeg", "ffprobe":
		return InstallFFmpeg(ctx, packageManagerRunner)
	default:
		return fmt.Errorf("unknown dependency %q", depName)
	}
}

// UpdateDependency updates a single dependency.
func UpdateDependency(ctx context.Context, depName string) error {
	binDir, err := EnsureManagedBinDir()
	if err != nil {
		return err
	}

	defer func() {
		if c, cErr := cache.New(); cErr == nil {
			_ = c.Delete(fmt.Sprintf("deps_%s.json", depName))
		}
	}()

	switch depName {
	case "fetch-track":
		return DownloadAndExtractGoBinary(ctx, "alexgorbatchev", "fetch-track-cli", "fetch-track", binDir)
	case "yt-dlp":
		return UpdateYtDlp(ctx, packageManagerRunner, binDir)
	case "ffmpeg", "ffprobe":
		return InstallFFmpeg(ctx, packageManagerRunner)
	default:
		return fmt.Errorf("unknown dependency %q", depName)
	}
}

// InstallMissingDependencies checks and installs all missing or unsatisfied dependencies.
func InstallMissingDependencies(ctx context.Context) ([]string, error) {
	_ = InitManagedPath()

	reports, _ := VerifyDependencies(ctx)
	var installed []string
	var errs []error

	for _, r := range reports {
		if !r.Satisfied {
			if err := InstallDependency(ctx, r.Name); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
			} else {
				installed = append(installed, r.Name)
			}
		}
	}

	if len(errs) > 0 {
		return installed, errors.Join(errs...)
	}

	return installed, nil
}

// UpdateAllDependencies updates all supported dependencies to their latest versions.
func UpdateAllDependencies(ctx context.Context) ([]string, error) {
	_ = InitManagedPath()

	var updated []string
	var errs []error

	for _, dep := range RequiredDependencies {
		if err := UpdateDependency(ctx, dep.Name); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dep.Name, err))
		} else {
			updated = append(updated, dep.Name)
		}
	}

	if len(errs) > 0 {
		return updated, errors.Join(errs...)
	}

	return updated, nil
}
