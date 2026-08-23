package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/cmdutil"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/llm"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

type CommentCandidate struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	LikeCount int    `json:"like_count"`
}

type youtubeComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	LikeCount int    `json:"like_count"`
}

type youtubeVideoInfo struct {
	Title    string           `json:"title"`
	Comments []youtubeComment `json:"comments"`
}

type CachedYouTubeTracks struct {
	VideoID      string              `json:"videoId"`
	VideoTitle   string              `json:"videoTitle"`
	Tracks       []types.Track       `json:"tracks"`
	SkippedItems []types.SkippedItem `json:"skippedItems,omitempty"`
}

var (
	rxVideoID = regexp.MustCompile(`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/|shorts\/)([^"&?\/\s]{11})`)
	rxDashes  = regexp.MustCompile(`[\-\x{2012}\x{2013}\x{2014}\x{2015}:]`)
	rxDigits  = regexp.MustCompile(`\d`)
)

// ExtractVideoID extracts the 11-character YouTube video ID from a URL or raw ID.
func ExtractVideoID(urlOrID string) string {
	trimmed := strings.TrimSpace(urlOrID)
	if len(trimmed) == 11 && !strings.Contains(trimmed, "/") && !strings.Contains(trimmed, ".") && !strings.Contains(trimmed, " ") {
		return trimmed
	}
	match := rxVideoID.FindStringSubmatch(trimmed)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// PreFilterComments selects comments that look like tracklist candidates.
func PreFilterComments(comments []youtubeComment) []CommentCandidate {
	var candidates []CommentCandidate

	for _, c := range comments {
		text := c.Text
		lines := strings.Split(text, "\n")
		if len(lines) < 3 {
			continue
		}

		dashesCount := len(rxDashes.FindAllString(text, -1))
		digitsCount := len(rxDigits.FindAllString(text, -1))

		if dashesCount >= 3 && digitsCount >= 5 {
			candidates = append(candidates, CommentCandidate{
				ID:        c.ID,
				Author:    c.Author,
				Text:      text,
				LikeCount: c.LikeCount,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LikeCount > candidates[j].LikeCount
	})

	return candidates
}

// ExtractTracklistWithAI builds the extraction prompt and queries the configured LLM provider.
func ExtractTracklistWithAI(ctx context.Context, candidates []CommentCandidate, providerID, modelName string) (*llm.GeminiResult, error) {
	var sb strings.Builder
	sb.WriteString("You are an expert DJ tracklist extractor.\n")
	sb.WriteString("Below is a list of candidate comments from a YouTube DJ set video.\n")
	sb.WriteString("Identify which comment represents the complete DJ set tracklist (with track names and artists).\n")
	sb.WriteString("Extract valid tracks (with artist and title) into \"tracks\".\n")
	sb.WriteString("Also identify any non-track lines in that comment (such as speech/commentary lines like \"Thanks speech\", empty timestamps, intro/outro markers, or incomplete entries missing an artist name) and return them in \"skippedItems\" with \"timestamp\", \"rawText\", and \"reason\" (e.g. \"speech\", \"incomplete\", \"marker\", \"empty\").\n\nCandidates:\n")

	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("--- CANDIDATE %d (ID: %s, Author: %s) ---\n%s\n\n", i+1, c.ID, c.Author, c.Text))
	}

	sb.WriteString("If no candidate represents a full tracklist, return JSON: {\"found\": false}.\n")
	sb.WriteString("Otherwise, return valid JSON with: {\"found\": true, \"commentId\": \"...\", \"tracks\": [{\"artist\": \"...\", \"title\": \"...\", \"timestamp\": \"...\"}], \"skippedItems\": [{\"timestamp\": \"...\", \"rawText\": \"...\", \"reason\": \"...\"}]}")

	prompt := sb.String()
	return llm.ExtractTracklistWithAI(ctx, providerID, modelName, prompt)
}

// FetchVideoComments retrieves video info and up to 500 comments via yt-dlp, using local cache if available.
func FetchVideoComments(ctx context.Context, videoURL string, noCache bool) (*youtubeVideoInfo, bool, error) {
	c, err := cache.New()
	if err != nil {
		return nil, false, err
	}

	videoID := ExtractVideoID(videoURL)
	cacheKey := fmt.Sprintf("yt_cache_%s.json", videoID)

	if videoID != "" && !noCache {
		var info youtubeVideoInfo
		if c.Get(cacheKey, &info) {
			return &info, true, nil
		}
	}

	fmt.Println("Fetching video details and comments from YouTube (up to 500 comments)...")

	// Run yt-dlp to fetch comments
	args := []string{
		"--no-playlist",
		"--write-comments",
		"--skip-download",
		"--dump-json",
		"--extractor-args", "youtube:max-comments=200,comment_sort=top",
		"--js-runtimes", "node",
		"--no-update",
		videoURL,
	}

	cmd := cmdutil.NewCommand(ctx, "yt-dlp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, fmt.Errorf("yt-dlp comment extraction failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var info youtubeVideoInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, false, fmt.Errorf("failed to parse yt-dlp JSON output: %w", err)
	}

	if videoID != "" && !noCache {
		_ = c.Put(cacheKey, info)
	}

	return &info, false, nil
}

// ProcessYouTubeComments runs full pipeline: fetch/cache comments, pre-filter, extract via LLM.
func ProcessYouTubeComments(ctx context.Context, videoURL string, noCache bool, providerID, modelName string) ([]types.Track, []types.SkippedItem, string, error) {
	c, err := cache.New()
	videoID := ExtractVideoID(videoURL)
	tracksCacheKey := fmt.Sprintf("yt_tracks_%s.json", videoID)

	if c != nil && videoID != "" && !noCache {
		var cachedCached CachedYouTubeTracks
		if c.Get(tracksCacheKey, &cachedCached) && len(cachedCached.Tracks) > 0 {
			fmt.Printf("Found cached tracklist on disk for YouTube video ID %q\n", videoID)
			return cachedCached.Tracks, cachedCached.SkippedItems, cachedCached.VideoTitle, nil
		}
	}

	info, commentCached, err := FetchVideoComments(ctx, videoURL, noCache)
	if err != nil {
		return nil, nil, "", err
	}

	if commentCached {
		fmt.Printf("Found cached comments on disk for YouTube video ID %q (%d comments)\n", videoID, len(info.Comments))
	} else {
		fmt.Printf("Retrieved %d comments for video %q\n", len(info.Comments), info.Title)
	}

	fmt.Println("Filtering comments for tracklist candidates...")
	candidates := PreFilterComments(info.Comments)
	if len(candidates) == 0 {
		return nil, nil, info.Title, fmt.Errorf("no comments found that look like a tracklist")
	}

	resolvedP, resolvedM, _ := llm.ResolveProvider(providerID, modelName)
	fmt.Printf("Found %d candidate comments. Querying LLM provider %s (%s) for tracklist extraction...\n", len(candidates), resolvedP, resolvedM)

	llmResult, err := ExtractTracklistWithAI(ctx, candidates, providerID, modelName)
	if err != nil {
		return nil, nil, info.Title, fmt.Errorf("LLM tracklist extraction failed: %w", err)
	}

	if !llmResult.Found || len(llmResult.Tracks) == 0 {
		return nil, nil, info.Title, fmt.Errorf("LLM could not identify a valid tracklist in the comment candidates")
	}

	var tracks []types.Track
	for _, gt := range llmResult.Tracks {
		artist := strings.TrimSpace(gt.Artist)
		title := strings.TrimSpace(gt.Title)
		if artist != "" && title != "" {
			tracks = append(tracks, types.Track{
				Artist:    artist,
				Title:     title,
				RawString: fmt.Sprintf("%s - %s", artist, title),
				Timestamp: strings.TrimSpace(gt.Timestamp),
			})
		}
	}

	skippedItems := llmResult.SkippedItems

	if c != nil && videoID != "" && !noCache && len(tracks) > 0 {
		_ = c.Put(tracksCacheKey, CachedYouTubeTracks{
			VideoID:      videoID,
			VideoTitle:   info.Title,
			Tracks:       tracks,
			SkippedItems: skippedItems,
		})
	}

	return tracks, skippedItems, info.Title, nil
}
