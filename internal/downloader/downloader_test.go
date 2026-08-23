package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestNormalizeTimestamp(t *testing.T) {
	tests := []struct {
		input      string
		alignHours bool
		want       string
	}{
		{"55:39", true, "00:55:39"},
		{"01:02:50", true, "01:02:50"},
		{"0:30", true, "00:00:30"},
		{"55:39", false, "55:39"},
		{"01:02:50", false, "01:02:50"},
		{"", true, ""},
	}

	for _, tt := range tests {
		got := NormalizeTimestamp(tt.input, tt.alignHours)
		if got != tt.want {
			t.Errorf("NormalizeTimestamp(%q, %v) = %q, want %q", tt.input, tt.alignHours, got, tt.want)
		}
	}
}

func TestHasHourTimestamps(t *testing.T) {
	tracksWithHour := []types.Track{
		{Artist: "A", Title: "T1", Timestamp: "55:39"},
		{Artist: "B", Title: "T2", Timestamp: "01:02:50"},
	}

	tracksWithoutHour := []types.Track{
		{Artist: "A", Title: "T1", Timestamp: "00:30"},
		{Artist: "B", Title: "T2", Timestamp: "55:39"},
	}

	if !HasHourTimestamps(tracksWithHour, nil) {
		t.Errorf("HasHourTimestamps(tracksWithHour) = false, want true")
	}

	if HasHourTimestamps(tracksWithoutHour, nil) {
		t.Errorf("HasHourTimestamps(tracksWithoutHour) = true, want false")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Artist/Title", "Artist-Title"},
		{"Bicep? Essential: Mix*", "Bicep- Essential- Mix-"},
		{"<Illegal> | Characters \"Quote\"", "-Illegal- - Characters -Quote-"},
		{"Normal Artist - Normal Title", "Normal Artist - Normal Title"},
	}

	for _, tt := range tests {
		got := SanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateM3UPlaylist(t *testing.T) {
	tempDir := t.TempDir()
	mixTitle := "Polo & Pan live for Cercle"
	tracks := []types.Track{
		{Artist: "Vladimir Cosma", Title: "Sirba (Polo & Pan Edit)"},
		{Artist: "Barbatuques", Title: "Baião Destemperado (Polo & Pan Edit)"},
		{Artist: "Polo & Pan", Title: "Zoom Zoom"},
	}

	// Create dummy downloaded files in tempDir
	_ = os.WriteFile(filepath.Join(tempDir, "01 - Vladimir Cosma - Sirba (Polo & Pan Edit).m4a"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "02 - Barbatuques - Baião Destemperado (Polo & Pan Edit).m4a"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "03 - Polo & Pan - Zoom Zoom.m4a"), []byte("audio"), 0644)

	playlistPath, err := GenerateM3UPlaylist(tempDir, mixTitle, tracks, nil)
	if err != nil {
		t.Fatalf("GenerateM3UPlaylist() returned unexpected error: %v", err)
	}

	if filepath.Base(playlistPath) != "playlist.m3u" {
		t.Errorf("Expected playlist file name to be playlist.m3u, got %q", filepath.Base(playlistPath))
	}

	contentBytes, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("Failed to read generated playlist file: %v", err)
	}

	content := string(contentBytes)
	if !strings.HasPrefix(content, "#EXTM3U\n") {
		t.Errorf("Playlist content missing #EXTM3U header")
	}

	expectedLines := []string{
		"#EXTINF:-1,Vladimir Cosma - Sirba (Polo & Pan Edit)",
		"01 - Vladimir Cosma - Sirba (Polo & Pan Edit).m4a",
		"#EXTINF:-1,Barbatuques - Baião Destemperado (Polo & Pan Edit)",
		"02 - Barbatuques - Baião Destemperado (Polo & Pan Edit).m4a",
		"#EXTINF:-1,Polo & Pan - Zoom Zoom",
		"03 - Polo & Pan - Zoom Zoom.m4a",
	}

	for _, line := range expectedLines {
		if !strings.Contains(content, line) {
			t.Errorf("Playlist file missing expected line %q. Content:\n%s", line, content)
		}
	}
}

func TestDownloadSetDryRun(t *testing.T) {
	opts := DownloadOptions{
		MixTitle:  "Bicep Essential Mix 2014",
		Tracks:    []types.Track{{Artist: "Bicep", Title: "Track 1", RawString: "Bicep - Track 1"}},
		OutputDir: "",
		DryRun:    true,
	}

	err := DownloadSet(context.Background(), opts)
	if err != nil {
		t.Fatalf("DownloadSet() in dry-run mode returned error: %v", err)
	}
}
