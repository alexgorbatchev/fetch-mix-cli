package search

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

const (
	defaultUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultHTTPTimeout = 15 * time.Second
)

var (
	mixesDBBaseURL   = "https://www.mixesdb.com"
	openingTrackAPI  = "https://www.openingtrack.com/wp-json/wp/v2/posts"
	tracklistClubAPI = "https://tracklist.club/wp-json/wp/v2/posts"
)

type mixesDBSearchResponse struct {
	Query struct {
		Search []struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"search"`
	} `json:"query"`
}

type wpPost struct {
	ID    int    `json:"id"`
	Link  string `json:"link"`
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
}

// IsCrawlableURL checks if a URL is from a known crawlable tracklist site.
func IsCrawlableURL(urlStr string) bool {
	u := strings.ToLower(urlStr)
	isMixesDb := strings.Contains(u, "mixesdb.com/w/") &&
		!strings.Contains(u, "category:") &&
		!strings.Contains(u, "special:") &&
		!strings.Contains(u, "talk:") &&
		!strings.Contains(u, "user:")

	return isMixesDb ||
		strings.Contains(u, "openingtrack.com/") ||
		strings.Contains(u, "thomaslaupstad.com/") ||
		strings.Contains(u, "tracklist.club/")
}

// SearchMixesDB searches MixesDB via MediaWiki API.
func SearchMixesDB(ctx context.Context, query string) ([]types.SearchResult, error) {
	reqURL := fmt.Sprintf("%s/w/api.php?action=query&list=search&srsearch=%s&format=json",
		mixesDBBaseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating MixesDB search request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MixesDB search HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MixesDB search returned status %d", resp.StatusCode)
	}

	var parsed mixesDBSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parsing MixesDB search response: %w", err)
	}

	var results []types.SearchResult
	for _, item := range parsed.Query.Search {
		title := item.Title
		pageSlug := strings.ReplaceAll(title, " ", "_")
		pageURL := fmt.Sprintf("%s/w/%s", mixesDBBaseURL, url.PathEscape(pageSlug))
		results = append(results, types.SearchResult{
			Title: title,
			URL:   pageURL,
		})
	}

	return results, nil
}

func searchWordPressEndpoint(ctx context.Context, client *http.Client, endpoint, query string) ([]types.SearchResult, error) {
	reqURL := fmt.Sprintf("%s?search=%s&per_page=5", endpoint, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint %s returned status %d", endpoint, resp.StatusCode)
	}

	var posts []wpPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, err
	}

	var results []types.SearchResult
	for _, p := range posts {
		title := html.UnescapeString(p.Title.Rendered)
		results = append(results, types.SearchResult{
			Title: title,
			URL:   p.Link,
		})
	}
	return results, nil
}

// SearchMirrors searches supported WordPress-based tracklist mirrors.
func SearchMirrors(ctx context.Context, query string) ([]types.SearchResult, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	var allResults []types.SearchResult

	if otResults, err := searchWordPressEndpoint(ctx, client, openingTrackAPI, query); err == nil {
		allResults = append(allResults, otResults...)
	}

	if tcResults, err := searchWordPressEndpoint(ctx, client, tracklistClubAPI, query); err == nil {
		allResults = append(allResults, tcResults...)
	}

	return allResults, nil
}

// SearchSet searches for a DJ set's tracklist across MixesDB and crawlable mirrors.
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
	mixesResults, err := SearchMixesDB(ctx, finalQuery)
	if err == nil && len(mixesResults) > 0 {
		return mixesResults, nil
	}

	// 2. Fall back to mirror search
	mirrorResults, err := SearchMirrors(ctx, finalQuery)
	if err == nil && len(mirrorResults) > 0 {
		return mirrorResults, nil
	}

	return nil, nil
}
