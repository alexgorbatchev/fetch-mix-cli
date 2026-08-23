package deps

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexgorbatchev/godeps"
)

// UpgradeSelf checks for a newer GitHub release and replaces the binary in-place.
func UpgradeSelf(ctx context.Context, currentVersion string) (bool, string, error) {
	latestTag, err := godeps.ResolveLatestTag(ctx, "alexgorbatchev", "fetch-mix-cli")
	if err != nil {
		return false, "", fmt.Errorf("resolving latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(latestTag, "v")
	cleanCurrent := strings.TrimPrefix(currentVersion, "v")

	if cleanCurrent != "dev" && cleanCurrent != "" {
		if compErr := godeps.CompareVersions(cleanCurrent, latestVersion); compErr == nil {
			return false, latestVersion, nil
		}
	}

	newVer, err := godeps.UpgradeSelf(ctx, "alexgorbatchev", "fetch-mix-cli", currentVersion)
	if err != nil {
		return false, "", err
	}
	return true, newVer, nil
}

// UpgradeSelfToPath downloads the latest release binary and replaces the binary at destPath.
func UpgradeSelfToPath(ctx context.Context, currentVersion, destPath string) (bool, string, error) {
	latestTag, err := godeps.ResolveLatestTag(ctx, "alexgorbatchev", "fetch-mix-cli")
	if err != nil {
		return false, "", fmt.Errorf("resolving latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(latestTag, "v")
	cleanCurrent := strings.TrimPrefix(currentVersion, "v")

	if cleanCurrent != "dev" && cleanCurrent != "" {
		if compErr := godeps.CompareVersions(cleanCurrent, latestVersion); compErr == nil {
			return false, latestVersion, nil
		}
	}

	newVer, err := godeps.UpgradeSelfToPath(ctx, "alexgorbatchev", "fetch-mix-cli", currentVersion, destPath)
	if err != nil {
		return false, "", err
	}
	return true, newVer, nil
}
