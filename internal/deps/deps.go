package deps

import (
	"context"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
	"github.com/alexgorbatchev/godeps"
)

type Dependency = godeps.Dependency
type DependencyReport = godeps.DependencyReport
type CommandRunner = godeps.CommandRunner

var DefaultRunner = godeps.DefaultRunner

var RequiredDependencies = []Dependency{
	{
		Name:       "fetch-track",
		MinVersion: "1.4.0",
		InstallURL: "https://github.com/alexgorbatchev/fetch-track-cli",
		Installer:  godeps.GitHubReleaseGoBinary("alexgorbatchev", "fetch-track-cli", "fetch-track"),
	},
	{
		Name:       "yt-dlp",
		MinVersion: "2024.08.01",
		InstallURL: "https://github.com/yt-dlp/yt-dlp#installation",
		Installer:  godeps.YtDlp(),
	},
	{
		Name:        "ffmpeg",
		MinVersion:  "4.4",
		InstallURL:  "https://ffmpeg.org/download.html",
		VersionArgs: []string{"-version"},
		Installer:   godeps.SystemPackageManager("ffmpeg"),
	},
}

// NewManager creates a godeps.Manager configured for fetch-mix.
func NewManager(c ...godeps.Cache) *godeps.Manager {
	var cacheInst godeps.Cache
	if len(c) > 0 && c[0] != nil {
		cacheInst = c[0]
	} else if defCache, err := cache.New(); err == nil {
		cacheInst = defCache
	}
	return godeps.New(godeps.Config{
		AppName:      "fetch-mix",
		Dependencies: RequiredDependencies,
		Cache:        cacheInst,
	})
}

func CheckDependencies(ctx context.Context, cacheInst ...*cache.Cache) error {
	var c godeps.Cache
	if len(cacheInst) > 0 && cacheInst[0] != nil {
		c = cacheInst[0]
	}
	return NewManager(c).Ensure(ctx)
}

func CheckDependenciesWithRunner(ctx context.Context, runner CommandRunner, c *cache.Cache, deps ...Dependency) error {
	mgrDeps := deps
	if len(mgrDeps) == 0 {
		mgrDeps = RequiredDependencies
	}
	var cacheInst godeps.Cache
	if c != nil {
		cacheInst = c
	}
	mgr := godeps.New(godeps.Config{
		AppName:      "fetch-mix",
		Dependencies: mgrDeps,
		Runner:       runner,
		Cache:        cacheInst,
	})
	_, err := mgr.Verify(ctx)
	return err
}

func VerifyDependencies(ctx context.Context, cacheInst ...*cache.Cache) ([]DependencyReport, error) {
	var c godeps.Cache
	if len(cacheInst) > 0 && cacheInst[0] != nil {
		c = cacheInst[0]
	}
	return NewManager(c).Verify(ctx)
}

func VerifyDependenciesWithRunner(ctx context.Context, runner CommandRunner, c *cache.Cache, deps ...Dependency) ([]DependencyReport, error) {
	mgrDeps := deps
	if len(mgrDeps) == 0 {
		mgrDeps = RequiredDependencies
	}
	var cacheInst godeps.Cache
	if c != nil {
		cacheInst = c
	}
	mgr := godeps.New(godeps.Config{
		AppName:      "fetch-mix",
		Dependencies: mgrDeps,
		Runner:       runner,
		Cache:        cacheInst,
	})
	return mgr.Verify(ctx)
}

func ParseVersionOutput(depName, output string) string {
	return godeps.DefaultVersionParser(depName, output)
}

func CompareVersions(actualStr, minStr string) error {
	return godeps.CompareVersions(actualStr, minStr)
}
