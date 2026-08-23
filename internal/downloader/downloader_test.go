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
		{"00:15:30", false, "15:30"},
		{"invalid", false, "invalid"},
		{"invalid", true, "invalid"},
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

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		sec  float64
		want string
	}{
		{45, "0:45"},
		{300, "5:00"},
		{503, "8:23"},
		{3665, "1:01:05"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.sec)
		if got != tt.want {
			t.Errorf("formatDuration(%f) = %q, want %q", tt.sec, got, tt.want)
		}
	}
}

func TestDownloadSetDryRun(t *testing.T) {
	opts := DownloadOptions{
		MixTitle:     "Bicep Essential Mix 2014",
		Tracks:       []types.Track{{Artist: "Bicep", Title: "Track 1", RawString: "Bicep - Track 1", Timestamp: "00:00"}},
		SkippedItems: []types.SkippedItem{{Timestamp: "01:00", RawText: "Intro", Reason: "intro marker"}},
		OutputDir:    "",
		DryRun:       true,
		Sources:      "youtube",
	}

	err := DownloadSet(context.Background(), opts)
	if err != nil {
		t.Fatalf("DownloadSet() in dry-run mode returned error: %v", err)
	}
}

func TestDownloadSet_ExecutionAndResume(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "downloads")

	// Create mock fetch-track script that also sends progress events if --progress-target is provided
	mockScript := filepath.Join(tempDir, "mock_fetch_track.sh")
	scriptContent := `#!/bin/sh
out=""
track=""
progress=""
while [ $# -gt 0 ]; do
  case "$1" in
    --out-dir) out="$2"; shift 2 ;;
    --progress-target) progress="$2"; shift 2 ;;
    *) if [ -z "$track" ]; then track="$1"; fi; shift ;;
  esac
done

if [ "$track" = "fail" ]; then
  echo "Download failed error" >&2
  exit 1
fi

mkdir -p "$out"
echo "audio data" > "$out/Unprefixed Artist - Track 1.m4a"

# If progress target is unix socket, send events
case "$progress" in
  unix://*)
    sock="${progress#unix://}"
    if [ -S "$sock" ]; then
      printf '{"type":"phase_start","phase":"search"}\n{"type":"phase_start","phase":"download"}\n{"type":"phase_start","phase":"verify"}\n{"type":"phase_start","phase":"metadata"}\n{"type":"candidate_selected","candidate":{"title":"Test Video","source":"youtube","duration":200}}\n{"type":"complete","result":{"path":"Unprefixed Artist - Track 1.m4a","title":"Track 1","album":"Album 1","releaseYear":"2024","bandwidthRating":"High","bandwidthHz":20000,"suggestedGainDb":1.5}}\n' | nc -U "$sock" 2>/dev/null || true
    fi
    ;;
esac
`
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	oldCmd := fetchTrackCmdName
	oldDelay := trackDownloadDelay
	fetchTrackCmdName = mockScript
	trackDownloadDelay = 0
	defer func() {
		fetchTrackCmdName = oldCmd
		trackDownloadDelay = oldDelay
	}()

	ctx := context.Background()
	opts := DownloadOptions{
		MixTitle:  "Test Set",
		OutputDir: outDir,
		Tracks: []types.Track{
			{Artist: "Artist", Title: "Track 1", RawString: "Artist - Track 1"},
			{Artist: "Artist 2", Title: "Track 2", RawString: "Artist 2 - Track 2"},
		},
		Sources:      "youtube",
		SkipVerify:   true,
		SkipMetadata: true,
		Verbose:      true,
	}

	// 1. Initial download (agent mode)
	t.Setenv("AGENT", "1")
	if err := DownloadSet(ctx, opts); err != nil {
		t.Fatalf("DownloadSet agent mode failed: %v", err)
	}

	// 2. Resume download (should skip already downloaded track)
	if err := DownloadSet(ctx, opts); err != nil {
		t.Fatalf("DownloadSet resume failed: %v", err)
	}

	// 3. Interactive mode (agent=0)
	t.Setenv("AGENT", "0")
	opts.Verbose = false
	opts.OutputDir = filepath.Join(tempDir, "interactive_out")
	if err := DownloadSet(ctx, opts); err != nil {
		t.Fatalf("DownloadSet interactive mode failed: %v", err)
	}

	// 4. Failing track execution
	failOpts := DownloadOptions{
		MixTitle:  "Fail Set",
		OutputDir: filepath.Join(tempDir, "fail_out"),
		Tracks: []types.Track{
			{Artist: "fail", Title: "fail", RawString: "fail"},
		},
	}
	_ = DownloadSet(ctx, failOpts)

	// 5. Canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = DownloadSet(canceledCtx, opts)
}
