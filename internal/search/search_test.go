package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsCrawlableURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.mixesdb.com/w/2014-09-27_-_Bicep_-_Essential_Mix", true},
		{"https://mixesdb.com/w/Special:Search", false},
		{"https://mixesdb.com/w/Category:2014", false},
		{"https://mixesdb.com/w/User:Admin", false},
		{"https://mixesdb.com/w/Talk:Main_Page", false},
		{"https://www.openingtrack.com/polo-pan-the-lot-radio-set/", true},
		{"https://thomaslaupstad.com/set-123/", true},
		{"https://tracklist.club/kontor-sunset-chill/", true},
		{"https://brizm.dev/some-tracklist", false},
		{"https://1001tracklists.com/tracklist/123.html", false},
		{"https://example.com/other", false},
	}

	for _, tt := range tests {
		got := IsCrawlableURL(tt.url)
		if got != tt.want {
			t.Errorf("IsCrawlableURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestSearchMixesDB(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/w/api.php" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("srsearch")
		if q != "Bicep Essential Mix" {
			t.Errorf("expected query 'Bicep Essential Mix', got %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"query": {
				"search": [
					{
						"pageid": 68764,
						"title": "2014-09-27 - Bicep - Essential Mix",
						"snippet": "Essential Mix snippet"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	oldBase := mixesDBBaseURL
	mixesDBBaseURL = server.URL
	defer func() { mixesDBBaseURL = oldBase }()

	ctx := context.Background()
	results, err := SearchMixesDB(ctx, "Bicep Essential Mix")
	if err != nil {
		t.Fatalf("SearchMixesDB error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "2014-09-27 - Bicep - Essential Mix" {
		t.Errorf("unexpected title: %q", results[0].Title)
	}
	expectedURL := server.URL + "/w/2014-09-27_-_Bicep_-_Essential_Mix"
	if results[0].URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, results[0].URL)
	}
}

func TestSearchMixesDB_Errors(t *testing.T) {
	// Status error
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer errServer.Close()

	oldBase := mixesDBBaseURL
	mixesDBBaseURL = errServer.URL
	defer func() { mixesDBBaseURL = oldBase }()

	ctx := context.Background()
	if _, err := SearchMixesDB(ctx, "test"); err == nil {
		t.Errorf("expected error on 500 response")
	}

	// Bad JSON
	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer badJSONServer.Close()

	mixesDBBaseURL = badJSONServer.URL
	if _, err := SearchMixesDB(ctx, "test"); err == nil {
		t.Errorf("expected error on bad JSON response")
	}
}

func TestSearchMirrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{
				"id": 436,
				"link": "https://www.openingtrack.com/polo-pan-the-lot-radio-set/",
				"title": {
					"rendered": "Polo &#038; Pan &#8211; The Lot Radio Set"
				}
			}
		]`))
	}))
	defer server.Close()

	oldOT := openingTrackAPI
	oldTC := tracklistClubAPI
	openingTrackAPI = server.URL
	tracklistClubAPI = server.URL
	defer func() {
		openingTrackAPI = oldOT
		tracklistClubAPI = oldTC
	}()

	ctx := context.Background()
	results, err := SearchMirrors(ctx, "Polo & Pan")
	if err != nil {
		t.Fatalf("SearchMirrors error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Polo & Pan – The Lot Radio Set" {
		t.Errorf("HTML unescaping failed, got %q", results[0].Title)
	}
}

func TestSearchMirrors_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer server.Close()

	oldOT := openingTrackAPI
	oldTC := tracklistClubAPI
	openingTrackAPI = server.URL
	tracklistClubAPI = server.URL
	defer func() {
		openingTrackAPI = oldOT
		tracklistClubAPI = oldTC
	}()

	ctx := context.Background()
	results, err := SearchMirrors(ctx, "test")
	if err != nil {
		t.Fatalf("SearchMirrors should not fail completely on mirror error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on error, got %d", len(results))
	}

	client := &http.Client{}
	_, err = searchWordPressEndpoint(ctx, client, server.URL, "query")
	if err == nil {
		t.Errorf("expected error from 500 endpoint in searchWordPressEndpoint")
	}

	// Bad JSON
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer badServer.Close()
	_, err = searchWordPressEndpoint(ctx, client, badServer.URL, "query")
	if err == nil {
		t.Errorf("expected JSON decode error")
	}
}

func TestSearchSet_FallbackToMirrors(t *testing.T) {
	mixesServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"query": {"search": []}}`))
	}))
	defer mixesServer.Close()

	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 1, "link": "https://example.com/set", "title": {"rendered": "Mirror Set"}}]`))
	}))
	defer mirrorServer.Close()

	oldBase := mixesDBBaseURL
	oldOT := openingTrackAPI
	oldTC := tracklistClubAPI
	mixesDBBaseURL = mixesServer.URL
	openingTrackAPI = mirrorServer.URL
	tracklistClubAPI = mirrorServer.URL
	defer func() {
		mixesDBBaseURL = oldBase
		openingTrackAPI = oldOT
		tracklistClubAPI = oldTC
	}()

	ctx := context.Background()
	results, err := SearchSet(ctx, "some rare set")
	if err != nil {
		t.Fatalf("SearchSet error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results from mirror fallback")
	}
}

func TestSearchSet_1001TracklistsExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("srsearch")
		if q != "bicep essential mix 2014 09 27" && q != "raw-slug" && q != "raw slug" {
			t.Errorf("unexpected query %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"query": {
				"search": [
					{
						"pageid": 123,
						"title": "2014-09-27 - Bicep - Essential Mix",
						"snippet": "Test"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	oldBase := mixesDBBaseURL
	mixesDBBaseURL = server.URL
	defer func() { mixesDBBaseURL = oldBase }()

	ctx := context.Background()
	results, err := SearchSet(ctx, "https://www.1001tracklists.com/tracklist/2w1t59lk/bicep-essential-mix-2014-09-27.html")
	if err != nil {
		t.Fatalf("SearchSet error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Test 1001tracklists URL without .html
	results2, err := SearchSet(ctx, "https://www.1001tracklists.com/tracklist/raw-slug")
	if err != nil {
		t.Fatalf("SearchSet error: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results2))
	}
}

func TestSetBaseURLs_Helpers(t *testing.T) {
	oldMixes := mixesDBBaseURL
	oldOT := openingTrackAPI
	oldTC := tracklistClubAPI
	defer func() {
		SetMixesDBBaseURLForTest(oldMixes)
		SetMirrorsBaseURLForTest(oldOT, oldTC)
	}()

	SetMixesDBBaseURLForTest("https://test.mixesdb.com")
	SetMirrorsBaseURLForTest("https://test.openingtrack.com", "https://test.tracklist.club")

	if mixesDBBaseURL != "https://test.mixesdb.com" {
		t.Errorf("mixesDBBaseURL not updated")
	}
	if openingTrackAPI != "https://test.openingtrack.com" || tracklistClubAPI != "https://test.tracklist.club" {
		t.Errorf("mirror URLs not updated")
	}
}
