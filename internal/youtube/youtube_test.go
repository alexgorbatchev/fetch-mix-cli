package youtube

import (
	"reflect"
	"testing"
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
