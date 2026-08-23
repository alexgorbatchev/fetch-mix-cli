package downloader

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cmdutil"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/ui"
)

type DownloadOptions struct {
	MixTitle     string
	Tracks       []types.Track
	SkippedItems []types.SkippedItem
	OutputDir    string
	DryRun       bool
	Sources      string
	SkipVerify   bool
	SkipMetadata bool
	Verbose      bool
}

var rxInvalidChars = regexp.MustCompile(`[/\\?%*:|"<>]+`)

// SanitizeFilename cleans invalid operating system path characters.
func SanitizeFilename(name string) string {
	cleaned := rxInvalidChars.ReplaceAllString(name, "-")
	return strings.TrimSpace(cleaned)
}

// NormalizeTimestamp formats a timestamp string.
// If alignHours is true, timestamps are formatted as "00:00:00".
// If alignHours is false, timestamps without hours are formatted as "00:00".
func NormalizeTimestamp(ts string, alignHours bool) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return ""
	}

	parts := strings.Split(ts, ":")
	if alignHours {
		var h, m, s int
		if len(parts) == 3 {
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &m)
			fmt.Sscanf(parts[2], "%d", &s)
		} else if len(parts) == 2 {
			h = 0
			fmt.Sscanf(parts[0], "%d", &m)
			fmt.Sscanf(parts[1], "%d", &s)
		} else {
			return ts
		}
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}

	var h, m, s int
	if len(parts) == 3 {
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &s)
		if h > 0 {
			return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		}
		return fmt.Sprintf("%02d:%02d", m, s)
	} else if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &s)
		return fmt.Sprintf("%02d:%02d", m, s)
	}

	return ts
}

// HasHourTimestamps checks if any track or skipped item in the set has an hour component in its timestamp.
func HasHourTimestamps(tracks []types.Track, skipped []types.SkippedItem) bool {
	checkTS := func(ts string) bool {
		parts := strings.Split(strings.TrimSpace(ts), ":")
		if len(parts) == 3 {
			var h int
			if _, err := fmt.Sscanf(parts[0], "%d", &h); err == nil && h > 0 {
				return true
			}
		}
		return false
	}

	for _, tr := range tracks {
		if checkTS(tr.Timestamp) {
			return true
		}
	}
	for _, item := range skipped {
		if checkTS(item.Timestamp) {
			return true
		}
	}

	return false
}

// GenerateM3UPlaylist creates an M3U playlist file named playlist.m3u preserving the set track order based on actual downloaded files in targetDir.
func GenerateM3UPlaylist(targetDir, mixTitle string, tracks []types.Track, manifest *MixManifest) (string, error) {
	playlistPath := filepath.Join(targetDir, "playlist.m3u")

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")

	// Scan manifest / targetDir for actual filenames
	actualFiles := make(map[int]string)
	if manifest != nil {
		for _, entry := range manifest.Tracks {
			if entry.ActualFile != "" {
				actualFiles[entry.Index] = entry.ActualFile
			}
		}
	}

	entries, _ := os.ReadDir(targetDir)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "playlist.m3u" || entry.Name() == "mix_manifest.json" || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		if len(name) >= 5 && name[2] == ' ' && name[3] == '-' && name[4] == ' ' {
			var trackNum int
			if _, err := fmt.Sscanf(name[:2], "%d", &trackNum); err == nil && trackNum > 0 {
				if _, exists := actualFiles[trackNum]; !exists {
					actualFiles[trackNum] = name
				}
			}
		}
	}

	for i, track := range tracks {
		trackNum := i + 1
		safeArtist := SanitizeFilename(track.Artist)
		safeTitle := SanitizeFilename(track.Title)

		actualFilename := actualFiles[trackNum]
		if actualFilename == "" {
			actualFilename = fmt.Sprintf("%02d - %s - %s", trackNum, safeArtist, safeTitle)
		}

		sb.WriteString(fmt.Sprintf("#EXTINF:-1,%s - %s\n", track.Artist, track.Title))
		sb.WriteString(fmt.Sprintf("%s\n", actualFilename))
	}

	if err := os.WriteFile(playlistPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write M3U playlist file: %w", err)
	}

	return playlistPath, nil
}

// DownloadSet downloads tracks in a set using fetch-track CLI as dependency.
func DownloadSet(ctx context.Context, opts DownloadOptions) error {
	safeMixTitle := SanitizeFilename(opts.MixTitle)
	if safeMixTitle == "" {
		safeMixTitle = "Downloaded DJ Set"
	}

	targetDir := opts.OutputDir
	if targetDir == "" || targetDir == "." {
		// Output by default goes into cwd/{mix-title-from-youtube}/...
		targetDir = safeMixTitle
	}

	playlistPath := filepath.Join(targetDir, "playlist.m3u")
	totalTracks := len(opts.Tracks)
	sep := ui.Separator("=", 50)
	hasHours := HasHourTimestamps(opts.Tracks, opts.SkippedItems)

	if opts.DryRun {
		fmt.Printf("\n%s\n", sep)
		fmt.Println("DRY RUN PREVIEW - No files will be downloaded")
		fmt.Printf("%s\n", sep)
		fmt.Printf("Mix / Set Title   : %s\n", opts.MixTitle)
		fmt.Printf("Target Directory  : %s/\n", targetDir)
		fmt.Printf("Playlist File     : %s\n", playlistPath)
		fmt.Printf("Total Tracks      : %d\n", totalTracks)
		if opts.Sources != "" {
			fmt.Printf("Sources           : %s\n", opts.Sources)
		}
		fmt.Println("\nPlanned Track Downloads:")

		for i, track := range opts.Tracks {
			trackNum := i + 1
			safeArtist := SanitizeFilename(track.Artist)
			safeTitle := SanitizeFilename(track.Title)
			trackTarget := fmt.Sprintf("%02d - %s - %s", trackNum, safeArtist, safeTitle)

			ts := NormalizeTimestamp(track.Timestamp, hasHours)
			if ts != "" {
				fmt.Printf("  [%s] %s\n", ts, trackTarget)
			} else {
				fmt.Printf("  %s\n", trackTarget)
			}
		}

		if len(opts.SkippedItems) > 0 {
			fmt.Printf("\nSkipped / Non-Track Items (%d):\n", len(opts.SkippedItems))
			for _, item := range opts.SkippedItems {
				tsStr := NormalizeTimestamp(item.Timestamp, hasHours)
				ts := ""
				if tsStr != "" {
					ts = fmt.Sprintf("[%s] ", tsStr)
				}
				reason := ""
				if item.Reason != "" {
					reason = fmt.Sprintf(" (%s)", item.Reason)
				}
				fmt.Printf("  - %s%s%s\n", ts, item.RawText, reason)
			}
		}

		fmt.Printf("\n%s\n", sep)
		fmt.Printf("Playlist file preview: %s (%d tracks in mix order)\n", playlistPath, totalTracks)
		fmt.Println("[DRY RUN COMPLETE] Plan verified. No downloads performed.")
		fmt.Printf("%s\n", sep)
		return nil
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", targetDir, err)
	}

	// Load or create MixManifest for pause & resume capability
	manifest, manifestErr := LoadOrCreateManifest(targetDir, opts.MixTitle, opts.Tracks, opts.SkippedItems)
	if manifestErr != nil {
		fmt.Printf("Warning: failed to initialize mix manifest: %v\n", manifestErr)
	}

	fmt.Printf("\nOutput Directory: %s\n", targetDir)
	fmt.Printf("Starting sequential track downloads via fetch-track CLI...\n")

	successCount := 0
	failureCount := 0

	for i, track := range opts.Tracks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trackNum := i + 1

		if !deps.IsAgentMode() {
			fmt.Println(sep)
		}

		// Pause & Resume check: skip already completed tracks
		if manifest != nil && i < len(manifest.Tracks) {
			entry := manifest.Tracks[i]
			if entry.Status == "completed" && entry.ActualFile != "" {
				fileOnDisk := filepath.Join(targetDir, entry.ActualFile)
				if fi, err := os.Stat(fileOnDisk); err == nil && fi.Size() > 0 {
					fmt.Printf("[%02d/%02d] Skipping already downloaded track: %s\n", trackNum, totalTracks, entry.ActualFile)
					successCount++
					continue
				}
			}
		}

		searchTarget := track.RawString
		if searchTarget == "" {
			searchTarget = fmt.Sprintf("%s - %s", track.Artist, track.Title)
		}

		tsStr := NormalizeTimestamp(track.Timestamp, hasHours)
		timestampInfo := ""
		if tsStr != "" {
			timestampInfo = fmt.Sprintf(" [%s]", tsStr)
		}

		fmt.Printf("[%02d/%02d] Fetching track: %s - %s%s\n", trackNum, totalTracks, track.Artist, track.Title, timestampInfo)

		// Take pre-download snapshot of targetDir files
		preSnapshot := SnapshotFiles(targetDir)

		args := []string{
			searchTarget,
			"--out-dir", targetDir,
		}

		if opts.Sources != "" {
			args = append(args, "--sources", opts.Sources)
		}
		if opts.SkipVerify {
			args = append(args, "--skip-verify")
		}
		if opts.SkipMetadata {
			args = append(args, "--skip-metadata")
		}
		if opts.Verbose {
			args = append(args, "--verbose")
		}

		cmd := cmdutil.NewCommand(ctx, "fetch-track", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Printf("  Failed to download track %q: %v\n", searchTarget, err)
			failureCount++

			if manifest != nil && i < len(manifest.Tracks) {
				manifest.Tracks[i].Status = "failed"
				manifest.Tracks[i].ErrorMessage = err.Error()
				_ = SaveManifest(manifest)
			}
		} else {
			// Detect newly created file
			newFile := DetectNewFile(targetDir, preSnapshot)
			finalFilename := newFile

			if newFile != "" {
				expectedPrefix := fmt.Sprintf("%02d - ", trackNum)
				if !strings.HasPrefix(newFile, expectedPrefix) {
					renamed := expectedPrefix + newFile
					oldPath := filepath.Join(targetDir, newFile)
					newPath := filepath.Join(targetDir, renamed)
					if err := os.Rename(oldPath, newPath); err == nil {
						finalFilename = renamed
					}
				}
			}

			fmt.Printf("  Completed: %s - %s\n", track.Artist, track.Title)
			successCount++

			if manifest != nil && i < len(manifest.Tracks) {
				manifest.Tracks[i].Status = "completed"
				manifest.Tracks[i].ActualFile = finalFilename
				_ = SaveManifest(manifest)
			}
		}

		if i < totalTracks-1 {
			delay := time.Duration(1000+rand.Intn(1500)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	// Generate M3U Playlist file in mix track order
	generatedPlaylist, err := GenerateM3UPlaylist(targetDir, opts.MixTitle, opts.Tracks, manifest)
	if err != nil {
		fmt.Printf("  Failed to generate playlist file: %v\n", err)
	} else {
		fmt.Printf("  Generated M3U playlist file: %s\n", generatedPlaylist)
	}

	fmt.Println(sep)
	fmt.Println("Download Process Finished!")
	fmt.Printf("  Downloaded: %d tracks\n", successCount)
	fmt.Printf("  Failed: %d tracks\n", failureCount)

	if failureCount > 0 && manifest != nil {
		fmt.Printf("\nFailed Tracks (%d):\n", failureCount)
		for _, entry := range manifest.Tracks {
			if entry.Status == "failed" {
				errMsg := ""
				if entry.ErrorMessage != "" {
					errMsg = fmt.Sprintf(" (%s)", entry.ErrorMessage)
				}
				fmt.Printf("  - [%02d/%02d] %s - %s%s\n", entry.Index, totalTracks, entry.Artist, entry.Title, errMsg)
			}
		}
	}
	fmt.Println(sep)

	return nil
}
