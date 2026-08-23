package downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestManifestSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	mixTitle := "Test Mix"
	tracks := []types.Track{
		{Artist: "Artist 1", Title: "Track 1", RawString: "Artist 1 - Track 1"},
		{Artist: "Artist 2", Title: "Track 2", RawString: "Artist 2 - Track 2"},
	}

	manifest, err := LoadOrCreateManifest(tempDir, mixTitle, tracks, nil)
	if err != nil {
		t.Fatalf("LoadOrCreateManifest failed: %v", err)
	}

	if len(manifest.Tracks) != 2 {
		t.Fatalf("Expected 2 track entries in manifest, got %d", len(manifest.Tracks))
	}

	manifest.Tracks[0].Status = "completed"
	manifest.Tracks[0].ActualFile = "01 - Artist 1 - Track 1.m4a"

	if err := SaveManifest(manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	loaded, err := LoadOrCreateManifest(tempDir, mixTitle, tracks, nil)
	if err != nil {
		t.Fatalf("LoadOrCreateManifest reload failed: %v", err)
	}

	if loaded.Tracks[0].Status != "completed" {
		t.Errorf("Expected track 0 status 'completed', got %q", loaded.Tracks[0].Status)
	}
	if loaded.Tracks[0].ActualFile != "01 - Artist 1 - Track 1.m4a" {
		t.Errorf("Expected actual file '01 - Artist 1 - Track 1.m4a', got %q", loaded.Tracks[0].ActualFile)
	}
}

func TestDetectNewFile(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "existing.txt"), []byte("data"), 0644)

	snapshot := SnapshotFiles(tempDir)
	if !snapshot["existing.txt"] {
		t.Errorf("Expected existing.txt in snapshot")
	}

	_ = os.WriteFile(filepath.Join(tempDir, "01 - new_track.m4a"), []byte("audio"), 0644)

	newFile := DetectNewFile(tempDir, snapshot)
	if newFile != "01 - new_track.m4a" {
		t.Errorf("Expected detected new file '01 - new_track.m4a', got %q", newFile)
	}
}
