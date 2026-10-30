package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestDownloadSetPlaylistFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "playlist.m3u"), 0700); err != nil {
		t.Fatal(err)
	}
	err := DownloadSet(context.Background(), DownloadOptions{MixTitle: "Empty set", OutputDir: dir})
	if err == nil || !strings.Contains(err.Error(), "playlist") {
		t.Fatalf("expected playlist failure, got %v", err)
	}
	if _, err := os.Stat(ManifestPath(dir)); err != nil {
		t.Fatalf("partial results missing: %v", err)
	}
}

func TestDownloadSetRejectsDamagedManifest(t *testing.T) {
	for _, data := range []string{`{broken`, `null`, `{}`, `{"tracks":"invalid"}`, `{"mixTitle":"Set","tracks":[{"index":1,"artist":"A","title":"B","status":"unknown"}]}`} {
		t.Run(data, func(t *testing.T) {
			dir := t.TempDir()
			path := ManifestPath(dir)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			err := DownloadSet(context.Background(), DownloadOptions{MixTitle: "Set", OutputDir: dir, Tracks: []types.Track{{Artist: "A", Title: "B"}}})
			if err == nil {
				t.Fatal("damaged manifest accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Fatalf("manifest changed: %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "playlist.m3u")); !os.IsNotExist(err) {
				t.Fatalf("playlist written despite invalid manifest: %v", err)
			}
		})
	}
}

func TestLoadManifestReadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(ManifestPath(dir), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateManifest(dir, "Set", nil, nil); err == nil || !strings.Contains(err.Error(), "reading mix manifest") {
		t.Fatalf("expected read failure, got %v", err)
	}
}
