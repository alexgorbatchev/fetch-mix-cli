package parser

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/llm"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

var (
	rxTracklistHeader  = regexp.MustCompile(`(?i)^tracklist`)
	rxSectionEnd       = regexp.MustCompile(`(?i)^(?:---|===|#|related mixes|comments|similar sets)`)
	rxMarkdownLink     = regexp.MustCompile(`\\?\[([^\]]+)\\?\]\(([^)]+)\)`)
	rxTimestampOnly    = regexp.MustCompile(`^(?:\d{1,2}:\d{2}(?::\d{2})?|▶\s*\d{1,2}:\d{2}|▶)$`)
	rxBrackets         = regexp.MustCompile(`\\?[\[{].*?\\?[\]}]`)
	rxPrefixLabel      = regexp.MustCompile(`(?i)^(?:intro\s*:\s*|outro\s*:\s*|track\s*\d+\s*:\s*)`)
	rxTrailingPlay     = regexp.MustCompile(`▶\s*\d{1,2}:\d{2}\s*$`)
	rxLeadingTimestamp = regexp.MustCompile(`(?i)^(?:\d{1,2}:\d{2}(?::\d{2})?|\[\d{1,2}:\d{2}\])\s*`)
	rxListPrefix       = regexp.MustCompile(`^(?:\d+[\.\s\-]+|\*|-)\s+`)
	rxTrimChars        = regexp.MustCompile(`^[\s_"?*▶•–—\-:]+|[\s_"?*▶•–—\-:]+$`)
	rxWhitespace       = regexp.MustCompile(`\s+`)
	rxDashSplit        = regexp.MustCompile(`\s+[\-\x{2012}\x{2013}\x{2014}\x{2015}]\s+`)
)

// ParseTracklist extracts structured track information and records skipped placeholder/intro/outro lines.
func ParseTracklist(markdown string) ([]types.Track, []types.SkippedItem) {
	lines := strings.Split(markdown, "\n")
	var tracks []types.Track
	var skipped []types.SkippedItem
	inTracklist := false

	hasTracklistSection := false
	for _, line := range lines {
		if rxTracklistHeader.MatchString(strings.TrimSpace(line)) {
			hasTracklistSection = true
			break
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if hasTracklistSection {
			if rxTracklistHeader.MatchString(trimmed) {
				inTracklist = true
				continue
			}

			if inTracklist {
				if rxSectionEnd.MatchString(trimmed) {
					if len(tracks) > 0 {
						break
					}
					continue
				}
			} else {
				continue
			}
		}

		// Extract timestamp if present in raw line
		timestampMatch := regexp.MustCompile(`^(?:\d{1,2}:\d{2}(?::\d{2})?|\[\d{1,2}:\d{2}\])`).FindString(trimmed)
		timestampStr := strings.Trim(timestampMatch, "[]")

		// 1. Clean markdown links: replace [Text](URL) with just Text, unless it's a timestamp
		processedLine := rxMarkdownLink.ReplaceAllStringFunc(trimmed, func(match string) string {
			sub := rxMarkdownLink.FindStringSubmatch(match)
			if len(sub) > 1 {
				linkText := strings.TrimSpace(sub[1])
				if rxTimestampOnly.MatchString(linkText) {
					return ""
				}
				return linkText
			}
			return ""
		})

		// 2. Remove standard brackets / braces representing labels or timestamp blocks
		processedLine = rxBrackets.ReplaceAllString(processedLine, "")

		// 2a. Strip leading intro/outro/track labels and trailing timestamp-play buttons
		processedLine = rxPrefixLabel.ReplaceAllString(processedLine, "")
		processedLine = rxTrailingPlay.ReplaceAllString(processedLine, "")

		// 3. Strip leading timestamps
		processedLine = rxLeadingTimestamp.ReplaceAllString(processedLine, "")

		// 4. Strip list items
		processedLine = rxListPrefix.ReplaceAllString(processedLine, "")

		// 5. Clean surrounding formatting
		processedLine = rxTrimChars.ReplaceAllString(processedLine, "")
		processedLine = strings.TrimSpace(processedLine)

		// 6. Normalize inner whitespace
		processedLine = rxWhitespace.ReplaceAllString(processedLine, " ")

		if processedLine == "" {
			continue
		}

		// 7. Extract Artist and Title
		parts := rxDashSplit.Split(processedLine, -1)
		if len(parts) >= 2 {
			artist := strings.TrimSpace(parts[0])
			title := strings.TrimSpace(strings.Join(parts[1:], " - "))

			if artist != "" && title != "" {
				lowerArtist := strings.ToLower(artist)
				lowerTitle := strings.ToLower(title)

				isPlaceholder := lowerArtist == "id" ||
					lowerTitle == "id" ||
					lowerArtist == "id-id" ||
					lowerTitle == "id-id" ||
					(lowerArtist == "untitled" && lowerTitle == "untitled") ||
					strings.Contains(lowerArtist, "id - id") ||
					strings.Contains(lowerTitle, "id - id") ||
					strings.Contains(lowerArtist, "untitled - untitled") ||
					strings.Contains(lowerTitle, "untitled - untitled")

				if lowerArtist == "intro" || lowerArtist == "outro" {
					skipped = append(skipped, types.SkippedItem{
						Timestamp: timestampStr,
						RawText:   processedLine,
						Reason:    lowerArtist + " marker",
					})
				} else if isPlaceholder {
					skipped = append(skipped, types.SkippedItem{
						Timestamp: timestampStr,
						RawText:   processedLine,
						Reason:    "unidentified track placeholder",
					})
				} else {
					tracks = append(tracks, types.Track{
						Artist:    artist,
						Title:     title,
						RawString: artist + " - " + title,
						Timestamp: timestampStr,
					})
				}
			} else {
				skipped = append(skipped, types.SkippedItem{
					Timestamp: timestampStr,
					RawText:   processedLine,
					Reason:    "incomplete track name",
				})
			}
		}
	}

	return tracks, skipped
}

// ParseTracklistWithAI extracts structured tracks using configured LLM provider,
// falling back to deterministic regex parsing if LLM is not configured or encounters an error.
func ParseTracklistWithAI(ctx context.Context, content, setTitle, providerID, modelName string) ([]types.Track, []types.SkippedItem, error) {
	// Attempt AI extraction first if LLM is available
	aiResult, err := llm.ExtractTracklistFromContentWithAI(ctx, providerID, modelName, content, setTitle)
	if err == nil && aiResult != nil && len(aiResult.Tracks) > 0 {
		var tracks []types.Track
		for _, tr := range aiResult.Tracks {
			raw := tr.Artist + " - " + tr.Title
			tracks = append(tracks, types.Track{
				Artist:    strings.TrimSpace(tr.Artist),
				Title:     strings.TrimSpace(tr.Title),
				RawString: strings.TrimSpace(raw),
				Timestamp: strings.TrimSpace(tr.Timestamp),
			})
		}
		return tracks, aiResult.SkippedItems, nil
	}

	// Fallback to deterministic regex-based parser
	detTracks, detSkipped := ParseTracklist(content)
	if len(detTracks) > 0 {
		return detTracks, detSkipped, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("AI tracklist parsing failed: %w", err)
	}
	return nil, nil, fmt.Errorf("could not extract any tracks from content")
}
