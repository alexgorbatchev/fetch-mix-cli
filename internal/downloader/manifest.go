package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

type TrackManifestEntry struct {
	Index         int     `json:"index"`
	Artist        string  `json:"artist"`
	Title         string  `json:"title"`
	Timestamp     string  `json:"timestamp,omitempty"`
	RawString     string  `json:"rawString"`
	Status        string  `json:"status"` // "pending", "completed", "failed"
	ActualFile    string  `json:"actualFile,omitempty"`
	Duration      float64 `json:"duration,omitempty"`
	BandwidthHz   int     `json:"bandwidthHz,omitempty"`
	QualityRating string  `json:"qualityRating,omitempty"`
	GainOffsetDb  float64 `json:"gainOffsetDb,omitempty"`
	SourceURL     string  `json:"sourceUrl,omitempty"`
	ErrorMessage  string  `json:"error,omitempty"`
}

type MixManifest struct {
	MixTitle  string               `json:"mixTitle"`
	TargetDir string               `json:"targetDir"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Tracks    []TrackManifestEntry `json:"tracks"`
	Skipped   []types.SkippedItem  `json:"skipped,omitempty"`
}

// ManifestPath returns the path to the mix manifest file inside targetDir.
func ManifestPath(targetDir string) string {
	return filepath.Join(targetDir, "mix_manifest.json")
}

// LoadOrCreateManifest loads an existing mix_manifest.json or creates a new one.
func LoadOrCreateManifest(targetDir, mixTitle string, tracks []types.Track, skipped []types.SkippedItem) (*MixManifest, error) {
	manifestFile := ManifestPath(targetDir)
	data, err := os.ReadFile(manifestFile)
	if err == nil {
		var m *MixManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("invalid mix manifest %q; repair it or choose a new --out-dir: %w", manifestFile, err)
		}
		if m == nil || m.Tracks == nil {
			return nil, fmt.Errorf("invalid mix manifest %q: missing tracks; repair it or choose a new --out-dir", manifestFile)
		}
		if !matchesTracklist(m, mixTitle, tracks) {
			return nil, fmt.Errorf("output directory contains a different tracklist; choose a new --out-dir to preserve existing downloads")
		}
		for _, entry := range m.Tracks {
			if entry.Status != "pending" && entry.Status != "completed" && entry.Status != "failed" {
				return nil, fmt.Errorf("invalid mix manifest %q: track %d has unknown status %q", manifestFile, entry.Index, entry.Status)
			}
		}
		m.TargetDir = targetDir
		m.Skipped = skipped
		return m, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading mix manifest %q: %w", manifestFile, err)
	}

	entries := make([]TrackManifestEntry, len(tracks))
	for i, tr := range tracks {
		entries[i] = TrackManifestEntry{
			Index:     i + 1,
			Artist:    tr.Artist,
			Title:     tr.Title,
			Timestamp: tr.Timestamp,
			RawString: tr.RawString,
			Status:    "pending",
		}
	}

	m := &MixManifest{
		MixTitle:  mixTitle,
		TargetDir: targetDir,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Tracks:    entries,
		Skipped:   skipped,
	}

	if err := SaveManifest(m); err != nil {
		return nil, err
	}

	return m, nil
}

func matchesTracklist(m *MixManifest, title string, tracks []types.Track) bool {
	if m.MixTitle != title || len(m.Tracks) != len(tracks) {
		return false
	}
	for i, tr := range tracks {
		entry := m.Tracks[i]
		if entry.Index != i+1 || entry.Artist != tr.Artist || entry.Title != tr.Title || entry.RawString != tr.RawString || entry.Timestamp != tr.Timestamp {
			return false
		}
	}
	return true
}

// SaveManifest writes the MixManifest to disk as targetDir/mix_manifest.json.
func SaveManifest(m *MixManifest) error {
	if m == nil || m.TargetDir == "" {
		return nil
	}
	m.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mix manifest: %w", err)
	}

	manifestFile := ManifestPath(m.TargetDir)
	if err := os.WriteFile(manifestFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write mix manifest %q: %w", manifestFile, err)
	}

	return nil
}

// SnapshotFiles returns a map of existing filenames in targetDir.
func SnapshotFiles(targetDir string) map[string]bool {
	files := make(map[string]bool)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != "mix_manifest.json" && entry.Name() != "playlist.m3u" && !strings.HasPrefix(entry.Name(), ".") {
			files[entry.Name()] = true
		}
	}
	return files
}

// DetectNewFile identifies a newly created file in targetDir by comparing against a pre-download snapshot.
func DetectNewFile(targetDir string, preSnapshot map[string]bool) string {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != "mix_manifest.json" && entry.Name() != "playlist.m3u" && !strings.HasPrefix(entry.Name(), ".") {
			if !preSnapshot[entry.Name()] {
				return entry.Name()
			}
		}
	}
	return ""
}
