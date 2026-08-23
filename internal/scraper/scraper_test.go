package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScrapeSet_HTMLToMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Tracklist</title></head>
			<body>
				<h1>Test Set Tracklist</h1>
				<h2>Tracklist</h2>
				<ul>
					<li>01. Artist 1 - Track 1</li>
					<li>02. Artist 2 - Track 2</li>
				</ul>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	ctx := context.Background()
	content, err := ScrapeSetWithCache(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("ScrapeSetWithCache error: %v", err)
	}

	if !strings.Contains(content, "Tracklist") {
		t.Errorf("expected markdown to contain 'Tracklist', got %q", content)
	}
	if !strings.Contains(content, "Artist 1 - Track 1") {
		t.Errorf("expected markdown to contain track text, got %q", content)
	}
}

func TestScrapeSet_MixesDBRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "action=raw") {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`== Tracklist ==
# [000] Intro
# [00?] Artist - Track`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	ctx := context.Background()
	// Test standard fetchPageContent against raw endpoint directly
	body, err := fetchHTTPBody(ctx, server.URL+"?action=raw")
	if err != nil {
		t.Fatalf("fetchHTTPBody error: %v", err)
	}

	if !strings.Contains(body, "== Tracklist ==") {
		t.Errorf("expected raw wikitext, got %q", body)
	}
}
