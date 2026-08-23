package youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"V19eCd8Odo8", "V19eCd8Odo8"},
		{"https://www.youtube.com/watch?v=V19eCd8Odo8", "V19eCd8Odo8"},
		{"https://youtu.be/V19eCd8Odo8?t=120", "V19eCd8Odo8"},
		{"https://www.youtube.com/shorts/V19eCd8Odo8", "V19eCd8Odo8"},
		{"invalid url", ""},
	}

	for _, tt := range tests {
		got := ExtractVideoID(tt.input)
		if got != tt.want {
			t.Errorf("ExtractVideoID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPreFilterComments(t *testing.T) {
	comments := []youtubeComment{
		{ID: "c1", Author: "User1", Text: "Great mix!", LikeCount: 100},
		{ID: "c2", Author: "User2", Text: "Tracklist:\n01. Artist - Title 1\n02. Artist - Title 2\n03. Artist - Title 3", LikeCount: 50},
	}

	candidates := PreFilterComments(comments)
	if len(candidates) != 1 {
		t.Fatalf("PreFilterComments() returned %d candidates, want 1", len(candidates))
	}

	if candidates[0].ID != "c2" {
		t.Errorf("PreFilterComments() candidate ID = %q, want %q", candidates[0].ID, "c2")
	}
}

func TestPreFilterCommentsSorting(t *testing.T) {
	comments := []youtubeComment{
		{ID: "low", Author: "User1", Text: "01: Artist - Track 1\n02: Artist - Track 2\n03: Artist - Track 3", LikeCount: 5},
		{ID: "high", Author: "User2", Text: "01: Artist - Track 1\n02: Artist - Track 2\n03: Artist - Track 3", LikeCount: 500},
	}

	candidates := PreFilterComments(comments)
	if len(candidates) != 2 {
		t.Fatalf("PreFilterComments() returned %d candidates, want 2", len(candidates))
	}

	wantIDs := []string{"high", "low"}
	gotIDs := []string{candidates[0].ID, candidates[1].ID}

	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("PreFilterComments() candidate order = %v, want %v", gotIDs, wantIDs)
	}
}

func TestProcessYouTubeComments_Integration(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Mock yt-dlp script
	mockScript := filepath.Join(tempDir, "mock_ytdlp.sh")
	ytdlpOutput := `{"title":"Live DJ Set 2024","comments":[{"id":"c1","author":"Fan","text":"Tracklist:\n01. Artist 1 - Track 1\n02. Artist 2 - Track 2\n03. Artist 3 - Track 3","like_count":100}]}`
	scriptContent := fmt.Sprintf("#!/bin/sh\ncat << 'EOF'\n%s\nEOF\n", ytdlpOutput)
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	oldYtDlp := ytdlpCmdName
	ytdlpCmdName = mockScript
	defer func() { ytdlpCmdName = oldYtDlp }()

	// Mock LLM server
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-yt",
			"object": "chat.completion",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"found\": true, \"commentId\": \"c1\", \"tracks\": [{\"artist\": \"Artist 1\", \"title\": \"Track 1\", \"timestamp\": \"00:00\"}, {\"artist\": \"Artist 2\", \"title\": \"Track 2\", \"timestamp\": \"05:00\"}], \"skippedItems\": []}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer llmServer.Close()

	t.Setenv("OPENAI_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test_key")

	ctx := context.Background()
	videoID := fmt.Sprintf("v%010d", time.Now().UnixNano()%10000000000)
	videoURL := "https://www.youtube.com/watch?v=" + videoID

	// 1. First run: fetches via mock yt-dlp and queries mock LLM
	tracks, skipped, title, err := ProcessYouTubeComments(ctx, videoURL, false, "custom", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("ProcessYouTubeComments failed: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
	if title != "Live DJ Set 2024" {
		t.Errorf("expected title 'Live DJ Set 2024', got %q", title)
	}
	if len(skipped) != 0 {
		t.Errorf("expected 0 skipped items")
	}

	// 2. Second run: disk cache hit
	cachedTracks, _, _, err := ProcessYouTubeComments(ctx, videoURL, false, "custom", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("ProcessYouTubeComments cache hit failed: %v", err)
	}
	if len(cachedTracks) != 2 {
		t.Errorf("expected 2 tracks from cache, got %d", len(cachedTracks))
	}
}

func TestProcessYouTubeComments_Errors(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Mock yt-dlp that outputs no comments
	mockScript := filepath.Join(tempDir, "mock_ytdlp_empty.sh")
	scriptContent := "#!/bin/sh\necho '{\"title\":\"No Comments Video\",\"comments\":[]}'\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	oldYtDlp := ytdlpCmdName
	ytdlpCmdName = mockScript
	defer func() { ytdlpCmdName = oldYtDlp }()

	ctx := context.Background()
	videoURL := "https://www.youtube.com/watch?v=11111111111"

	// Should error when no candidate comments found
	_, _, _, err := ProcessYouTubeComments(ctx, videoURL, true, "custom", "")
	if err == nil {
		t.Errorf("expected error when no candidate comments exist")
	}

	// Failing yt-dlp script
	mockFailScript := filepath.Join(tempDir, "mock_ytdlp_fail.sh")
	if err := os.WriteFile(mockFailScript, []byte("#!/bin/sh\necho 'error' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}

	ytdlpCmdName = mockFailScript
	_, _, _, err = ProcessYouTubeComments(ctx, videoURL, true, "custom", "")
	if err == nil {
		t.Errorf("expected error when yt-dlp fails")
	}
}

func TestExtractTracklistWithAI(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-yt",
			"object": "chat.completion",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"found\": true, \"commentId\": \"c1\", \"tracks\": [{\"artist\": \"Artist 1\", \"title\": \"Track 1\", \"timestamp\": \"00:00\"}], \"skippedItems\": []}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer llmServer.Close()

	t.Setenv("OPENAI_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test_key")

	candidates := []CommentCandidate{
		{ID: "c1", Author: "Fan", Text: "01. Artist 1 - Track 1\n02. Artist 2 - Track 2\n03. Artist 3 - Track 3", LikeCount: 10},
	}
	ctx := context.Background()
	res, err := ExtractTracklistWithAI(ctx, candidates, "custom", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("ExtractTracklistWithAI failed: %v", err)
	}
	if !res.Found || len(res.Tracks) != 1 {
		t.Errorf("unexpected AI extraction result: %#v", res)
	}
}
