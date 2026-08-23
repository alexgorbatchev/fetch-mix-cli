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
	if data, err := os.ReadFile(manifestFile); err == nil {
		var m MixManifest
		if json.Unmarshal(data, &m) == nil && len(m.Tracks) == len(tracks) {
			return &m, nil
		}
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
