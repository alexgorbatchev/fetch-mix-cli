package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

const (
	defaultUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultHTTPTimeout = 20 * time.Second
)

var (
	mixesDBHost = "mixesdb.com"
)

type cachedScrapedMarkdown struct {
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
}

// ScrapeSet scrapes a tracklist page to extract its markdown or wikitext content.
func ScrapeSet(ctx context.Context, urlStr string) (string, error) {
	return ScrapeSetWithCache(ctx, urlStr, false)
}

// ScrapeSetWithCache scrapes a set page, utilizing disk cache in .tmp/XDG cache unless noCache is true.
func ScrapeSetWithCache(ctx context.Context, urlStr string, noCache bool) (string, error) {
	c, err := cache.New()
	if err == nil && !noCache {
		cacheKey := fmt.Sprintf("scrape_%s.json", cache.HashKey(urlStr))
		var cached cachedScrapedMarkdown
		if c.Get(cacheKey, &cached) && cached.Markdown != "" {
			fmt.Printf("Found cached markdown on disk for URL %q\n", urlStr)
			return cached.Markdown, nil
		}
	}

	content, err := fetchPageContent(ctx, urlStr)
	if err != nil {
		return "", err
	}

	if c != nil && !noCache {
		cacheKey := fmt.Sprintf("scrape_%s.json", cache.HashKey(urlStr))
		_ = c.Put(cacheKey, cachedScrapedMarkdown{
			URL:      urlStr,
			Markdown: content,
		})
	}

	return content, nil
}

func fetchPageContent(ctx context.Context, urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", urlStr, err)
	}

	// MixesDB: fetch raw wikitext directly for 100% clean formatting
	if strings.Contains(strings.ToLower(u.Host), mixesDBHost) && strings.HasPrefix(u.Path, "/w/") {
		titleSlug := strings.TrimPrefix(u.Path, "/w/")
		if titleSlug != "" {
			rawURL := fmt.Sprintf("%s://%s/w/index.php?title=%s&action=raw", u.Scheme, u.Host, titleSlug)
			rawContent, rawErr := fetchHTTPBody(ctx, rawURL)
			if rawErr == nil && strings.TrimSpace(rawContent) != "" && !strings.HasPrefix(strings.TrimSpace(rawContent), "<!DOCTYPE") {
				return rawContent, nil
			}
		}
	}

	// General web page: fetch HTML and convert to Markdown
	htmlBody, err := fetchHTTPBody(ctx, urlStr)
	if err != nil {
		return "", fmt.Errorf("fetching URL %q: %w", urlStr, err)
	}

	markdown, err := htmltomarkdown.ConvertString(htmlBody)
	if err != nil {
		return htmlBody, nil // Fall back to raw text/html if markdown conversion fails
	}

	return strings.TrimSpace(markdown), nil
}

func fetchHTTPBody(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, targetURL)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	return string(bodyBytes), nil
}
