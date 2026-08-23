package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cmdutil"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

type firecrawlSearchOutput struct {
	Web []types.SearchResult `json:"web"`
}

// IsCrawlableURL checks if a URL is from a known crawlable tracklist site.
func IsCrawlableURL(urlStr string) bool {
	url := strings.ToLower(urlStr)
	isMixesDb := strings.Contains(url, "mixesdb.com/w/") &&
		!strings.Contains(url, "category:") &&
		!strings.Contains(url, "special:") &&
		!strings.Contains(url, "talk:") &&
		!strings.Contains(url, "user:")

	return isMixesDb ||
		strings.Contains(url, "openingtrack.com/") ||
		strings.Contains(url, "brizm.dev/") ||
		strings.Contains(url, "thomaslaupstad.com/") ||
		strings.Contains(url, "tracklist.club/")
}

func searchMixesDb(ctx context.Context, query string) ([]types.SearchResult, error) {
	searchQuery := query + " mixesdb"
	cmd := cmdutil.NewCommand(ctx, "firecrawl", "search", searchQuery, "--json")
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("firecrawl search failed: %w: %s", err, stderr.String())
	}

	outBytes := bytes.TrimSpace(stdout.Bytes())
	if len(outBytes) == 0 {
		outBytes = bytes.TrimSpace(stderr.Bytes())
	}
	if len(outBytes) == 0 {
		return nil, nil
	}

	var parsed firecrawlSearchOutput
	if err := json.Unmarshal(outBytes, &parsed); err != nil {
		return nil, nil
	}

	var filtered []types.SearchResult
	for _, res := range parsed.Web {
		u := strings.ToLower(res.URL)
		if strings.Contains(u, "mixesdb.com/w/") &&
			!strings.Contains(u, "category:") &&
			!strings.Contains(u, "special:") &&
			!strings.Contains(u, "talk:") &&
			!strings.Contains(u, "user:") {
			filtered = append(filtered, res)
		}
	}

	return filtered, nil
}

func searchGeneral(ctx context.Context, query string) ([]types.SearchResult, error) {
	cmd := cmdutil.NewCommand(ctx, "firecrawl", "search", query, "--json")
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("firecrawl search failed: %w: %s", err, stderr.String())
	}

	outBytes := bytes.TrimSpace(stdout.Bytes())
	if len(outBytes) == 0 {
		outBytes = bytes.TrimSpace(stderr.Bytes())
	}
	if len(outBytes) == 0 {
		return nil, nil
	}

	var parsed firecrawlSearchOutput
	if err := json.Unmarshal(outBytes, &parsed); err != nil {
		return nil, nil
	}

	var filtered []types.SearchResult
	for _, res := range parsed.Web {
		if IsCrawlableURL(res.URL) {
			filtered = append(filtered, res)
		}
	}

	return filtered, nil
}

// SearchSet searches for a DJ set's tracklist across MixesDB and general crawlable web mirrors.
func SearchSet(ctx context.Context, query string) ([]types.SearchResult, error) {
	finalQuery := query

	// 1001tracklists URL detection
	if strings.Contains(strings.ToLower(query), "1001tracklists.com/tracklist/") {
		urlParts := strings.Split(query, "/")
		var htmlPart string
		for _, p := range urlParts {
			if strings.HasSuffix(strings.ToLower(p), ".html") {
				htmlPart = p
				break
			}
		}
		if htmlPart != "" {
			slug := strings.TrimSuffix(htmlPart, ".html")
			finalQuery = strings.ReplaceAll(slug, "-", " ")
		} else if len(urlParts) > 0 {
			finalQuery = urlParts[len(urlParts)-1]
		}
	}

	// 1. Search MixesDB first
	mixesResults, err := searchMixesDb(ctx, finalQuery)
	if err == nil && len(mixesResults) > 0 {
		return mixesResults, nil
	}

	// 2. Fall back to general web search
	return searchGeneral(ctx, finalQuery)
}
