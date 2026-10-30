package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestResumeRequiresSameTracklist(t *testing.T) {
	for _, field := range []string{"artist", "title", "order", "query", "timestamp", "mix", "count"} {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			tracks := []types.Track{{Artist: "A", Title: "One", RawString: "A - One"}, {Artist: "B", Title: "Two"}}
			if _, err := LoadOrCreateManifest(dir, "Mix", tracks, nil); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(ManifestPath(dir))
			if err != nil {
				t.Fatal(err)
			}
			title := "Mix"
			switch field {
			case "artist":
				tracks[0].Artist = "Other"
			case "title":
				tracks[0].Title = "Other"
			case "order":
				tracks[0], tracks[1] = tracks[1], tracks[0]
			case "query":
				tracks[0].RawString = "remix"
			case "timestamp":
				tracks[0].Timestamp = "10:00"
			case "mix":
				title = "Other"
			case "count":
				tracks = tracks[:1]
			}
			if _, err := LoadOrCreateManifest(dir, title, tracks, nil); err == nil || !strings.Contains(err.Error(), "different tracklist") {
				t.Fatalf("expected mismatch, got %v", err)
			}
			after, err := os.ReadFile(ManifestPath(dir))
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("mismatched manifest overwritten")
			}
		})
	}
}

func TestResumeAfterMovingDirectory(t *testing.T) {
	parent := t.TempDir()
	oldDir, newDir := filepath.Join(parent, "old"), filepath.Join(parent, "new")
	if err := os.Mkdir(oldDir, 0755); err != nil {
		t.Fatal(err)
	}
	tracks := []types.Track{{Artist: "A", Title: "T"}}
	if _, err := LoadOrCreateManifest(oldDir, "Mix", tracks, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	m, err := LoadOrCreateManifest(newDir, "Mix", tracks, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Tracks[0].Status = "failed"
	if err := SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ManifestPath(newDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status": "failed"`) {
		t.Fatalf("state not saved at current destination: %s", data)
	}
}

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
	manifest.Tracks[0].Duration = 480.5
	manifest.Tracks[0].BandwidthHz = 20000
	manifest.Tracks[0].QualityRating = "High Fidelity (>=18.5 kHz)"
	manifest.Tracks[0].GainOffsetDb = -2.5
	manifest.Tracks[0].SourceURL = "https://soundcloud.com/artist/track"

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
	if loaded.Tracks[0].BandwidthHz != 20000 {
		t.Errorf("Expected bandwidth 20000, got %d", loaded.Tracks[0].BandwidthHz)
	}
	if loaded.Tracks[0].QualityRating != "High Fidelity (>=18.5 kHz)" {
		t.Errorf("Expected quality rating 'High Fidelity (>=18.5 kHz)', got %q", loaded.Tracks[0].QualityRating)
	}
	if loaded.Tracks[0].GainOffsetDb != -2.5 {
		t.Errorf("Expected gain offset -2.5, got %f", loaded.Tracks[0].GainOffsetDb)
	}
	if loaded.Tracks[0].SourceURL != "https://soundcloud.com/artist/track" {
		t.Errorf("Expected source URL 'https://soundcloud.com/artist/track', got %q", loaded.Tracks[0].SourceURL)
	}
}

func TestDetectNewFile(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "existing.txt"), []byte("data"), 0644)

	snapshot := SnapshotFiles(tempDir)
	if !snapshot["existing.txt"] {
		t.Errorf("Expected existing.txt in snapshot")
	}

	// Add sub-directory and playlist.m3u (which should be ignored by DetectNewFile)
	_ = os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "playlist.m3u"), []byte("m3u"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "mix_manifest.json"), []byte("{}"), 0644)

	_ = os.WriteFile(filepath.Join(tempDir, "01 - new_track.m4a"), []byte("audio"), 0644)

	newFile := DetectNewFile(tempDir, snapshot)
	if newFile != "01 - new_track.m4a" {
		t.Errorf("Expected detected new file '01 - new_track.m4a', got %q", newFile)
	}

	// Non-existent directory
	emptySnap := SnapshotFiles(filepath.Join(tempDir, "non_existent"))
	if len(emptySnap) != 0 {
		t.Errorf("expected empty snapshot for non-existent dir")
	}
}

func TestLoadOrCreateManifest_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	manifestFile := filepath.Join(tempDir, "mix_manifest.json")
	_ = os.WriteFile(manifestFile, []byte("{corrupted json"), 0644)

	tracks := []types.Track{{Artist: "A", Title: "B"}}
	m, err := LoadOrCreateManifest(tempDir, "Title", tracks, nil)
	if err == nil || m != nil {
		t.Fatalf("expected damaged manifest to be rejected, got %v, %v", m, err)
	}
	data, err := os.ReadFile(manifestFile)
	if err != nil || string(data) != "{corrupted json" {
		t.Fatalf("damaged manifest was changed: %q, %v", data, err)
	}
}
