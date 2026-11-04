package deps

import (
	"context"

	"github.com/alexgorbatchev/godeps/v2"
)

// GetManagedBinDir returns the path to the fetch-mix managed binary directory.
func GetManagedBinDir() (string, error) {
	return godeps.GetManagedBinDir("fetch-mix")
}

// EnsureManagedBinDir creates the managed binary directory if it does not exist.
func EnsureManagedBinDir() (string, error) {
	return godeps.EnsureManagedBinDir("fetch-mix")
}

// InitManagedPath puts the managed directories on the process PATH so each
// required dependency resolves to the copy the pipeline uses: a copy elsewhere
// on PATH that meets the minimum version wins, and the managed copy takes over
// only when that copy is missing or outdated.
func InitManagedPath(ctx context.Context) error {
	return NewManager().InitManagedPath(ctx)
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
