package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

func TestScrapeSet_Wrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<h1>Title</h1><p>Artist - Track</p>"))
	}))
	defer server.Close()

	ctx := context.Background()
	content, err := ScrapeSet(ctx, server.URL)
	if err != nil {
		t.Fatalf("ScrapeSet error: %v", err)
	}
	if !strings.Contains(content, "Artist - Track") {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestScrapeSet_CacheHit(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	c, err := cache.New()
	if err != nil {
		t.Fatalf("cache.New() error: %v", err)
	}

	testURL := "https://example.com/cached-tracklist"
	cacheKey := fmt.Sprintf("scrape_%s.json", cache.HashKey(testURL))
	_ = c.Put(cacheKey, cachedScrapedMarkdown{
		URL:      testURL,
		Markdown: "# Cached Tracklist Content",
	})

	ctx := context.Background()
	content, err := ScrapeSetWithCache(ctx, testURL, false)
	if err != nil {
		t.Fatalf("ScrapeSetWithCache error: %v", err)
	}
	if content != "# Cached Tracklist Content" {
		t.Errorf("expected cached content, got %q", content)
	}
}

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

	oldHost := mixesDBHost
	u, _ := http.NewRequest("GET", server.URL, nil)
	mixesDBHost = u.URL.Host
	defer func() { mixesDBHost = oldHost }()

	ctx := context.Background()
	content, err := ScrapeSetWithCache(ctx, server.URL+"/w/2014-09-27_-_Bicep_-_Essential_Mix", true)
	if err != nil {
		t.Fatalf("ScrapeSetWithCache error: %v", err)
	}

	if !strings.Contains(content, "== Tracklist ==") {
		t.Errorf("expected raw wikitext, got %q", content)
	}
}

func TestScrapeSet_Errors(t *testing.T) {
	ctx := context.Background()

	// Invalid URL
	_, err := ScrapeSetWithCache(ctx, "http://invalid-domain-that-does-not-exist-12345678.org", true)
	if err == nil {
		t.Errorf("expected error for non-existent domain")
	}

	// 404 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err = ScrapeSetWithCache(ctx, server.URL, true)
	if err == nil {
		t.Errorf("expected error for 404 response")
	}

	_, err = fetchHTTPBody(ctx, server.URL)
	if err == nil {
		t.Errorf("expected error for 404 fetchHTTPBody")
	}

	// Invalid URL scheme
	_, err = ScrapeSetWithCache(ctx, "://invalid-scheme", true)
	if err == nil {
		t.Errorf("expected error for invalid scheme")
	}
}
