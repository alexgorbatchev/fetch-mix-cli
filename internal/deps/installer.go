package deps

import (
	"context"

	"github.com/alexgorbatchev/godeps"
)

// GetManagedBinDir returns the path to the fetch-mix managed binary directory.
func GetManagedBinDir() (string, error) {
	return godeps.GetManagedBinDir("fetch-mix")
}

// EnsureManagedBinDir creates the managed binary directory if it does not exist.
func EnsureManagedBinDir() (string, error) {
	return godeps.EnsureManagedBinDir("fetch-mix")
}

// InitManagedPath ensures the managed bin directory is prepended to process PATH.
func InitManagedPath() error {
	return godeps.InitManagedPath("fetch-mix")
}

// ResolveLatestTag queries the latest release tag for a GitHub repository.
func ResolveLatestTag(ctx context.Context, owner, repo string) (string, error) {
	return godeps.ResolveLatestTag(ctx, owner, repo)
}

// ResolveLatestTagWithBaseURL queries the latest release tag for a repository from a custom base URL.
func ResolveLatestTagWithBaseURL(ctx context.Context, baseURL, owner, repo string) (string, error) {
	return godeps.ResolveLatestTagWithBaseURL(ctx, baseURL, owner, repo)
}

// DownloadAndExtractGoBinary downloads and extracts a Go binary from GitHub releases.
func DownloadAndExtractGoBinary(ctx context.Context, owner, repo, binName, targetDir string) error {
	return godeps.DownloadAndExtractGoBinary(ctx, owner, repo, binName, targetDir)
}

// InstallYtDlp downloads the standalone yt-dlp binary into targetDir.
func InstallYtDlp(ctx context.Context, targetDir string) error {
	return godeps.InstallYtDlp(ctx, targetDir)
}

// UpdateYtDlp updates yt-dlp.
func UpdateYtDlp(ctx context.Context, runner CommandRunner, targetDir string) error {
	return godeps.UpdateYtDlp(ctx, runner, targetDir)
}

// InstallFFmpeg attempts to install ffmpeg using host package manager.
func InstallFFmpeg(ctx context.Context, runner CommandRunner) error {
	return godeps.InstallPackage(ctx, "ffmpeg", runner)
}

// InstallDependency downloads or installs a single dependency into the managed bin directory.
func InstallDependency(ctx context.Context, depName string) error {
	return NewManager().Install(ctx, depName)
}

// UpdateDependency updates a single dependency.
func UpdateDependency(ctx context.Context, depName string) error {
	return NewManager().Update(ctx, depName)
}

// InstallMissingDependencies checks and installs all missing or unsatisfied dependencies.
func InstallMissingDependencies(ctx context.Context) ([]string, error) {
	return NewManager().InstallUnsatisfied(ctx)
}

// UpdateAllDependencies updates all supported dependencies to their latest versions.
func UpdateAllDependencies(ctx context.Context) ([]string, error) {
	return NewManager().UpdateAll(ctx)
}
