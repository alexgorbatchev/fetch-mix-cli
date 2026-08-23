package deps

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpgradeSelf(t *testing.T) {
	ctx := context.Background()
	_, _, _ = UpgradeSelf(ctx, "dev")
}

func TestUpgradeSelfToPath(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "fetch-mix")

	_, _, _ = UpgradeSelfToPath(ctx, "dev", destPath)
}
