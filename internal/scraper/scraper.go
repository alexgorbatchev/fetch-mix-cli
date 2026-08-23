package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

type firecrawlScrapeOutput struct {
	Markdown string `json:"markdown"`
}

type cachedScrapedMarkdown struct {
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
}

// ScrapeSet scrapes a tracklist page using firecrawl to extract its markdown content.
func ScrapeSet(ctx context.Context, url string) (string, error) {
	return ScrapeSetWithCache(ctx, url, false)
}

// ScrapeSetWithCache scrapes a set page, utilizing disk cache in .tmp unless noCache is true.
func ScrapeSetWithCache(ctx context.Context, url string, noCache bool) (string, error) {
	c, err := cache.New()
	if err == nil && !noCache {
		cacheKey := fmt.Sprintf("scrape_%s.json", cache.HashKey(url))
		var cached cachedScrapedMarkdown
		if c.Get(cacheKey, &cached) && cached.Markdown != "" {
			fmt.Printf("Found cached markdown on disk for URL %q\n", url)
			return cached.Markdown, nil
		}
	}

	cmd := exec.CommandContext(ctx, "firecrawl", "scrape", url, "--json")
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	outBytes := bytes.TrimSpace(stdout.Bytes())
	if len(outBytes) == 0 {
		outBytes = bytes.TrimSpace(stderr.Bytes())
	}

	if len(outBytes) == 0 {
		if runErr != nil {
			return "", fmt.Errorf("firecrawl scrape failed: %w", runErr)
		}
		return "", fmt.Errorf("firecrawl scrape returned empty output")
	}

	var parsed firecrawlScrapeOutput
	if err := json.Unmarshal(outBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse firecrawl scrape JSON: %w (output: %q)", err, string(outBytes))
	}

	if parsed.Markdown == "" {
		return "", fmt.Errorf("no markdown content returned from firecrawl scrape")
	}

	if c != nil && !noCache {
		cacheKey := fmt.Sprintf("scrape_%s.json", cache.HashKey(url))
		_ = c.Put(cacheKey, cachedScrapedMarkdown{
			URL:      url,
			Markdown: parsed.Markdown,
		})
	}

	return parsed.Markdown, nil
}
