package downloader

import (
	"bytes"
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
	"github.com/alexgorbatchev/fetch-mix-cli/internal/progress"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/ui"
)

type DownloadOptions struct {
	MixTitle         string
	Tracks           []types.Track
	SkippedItems     []types.SkippedItem
	OutputDir        string
	DryRun           bool
	Sources          string
	SkipVerify       bool
	SkipMetadata     bool
	Verbose          bool
	ProgressReporter *progress.Reporter
}

var (
	rxInvalidChars     = regexp.MustCompile(`[/\\?%*:|"<>]+`)
	fetchTrackCmdName  = "fetch-track"
	trackDownloadDelay = 1000 * time.Millisecond
)

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

func formatDuration(sec float64) string {
	s := int(sec)
	m := s / 60
	s = s % 60
	h := m / 60
	m = m % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
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
	isAgent := deps.IsAgentMode()

	for i, track := range opts.Tracks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trackNum := i + 1

		if !isAgent {
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

		baseArgs := []string{
			searchTarget,
			"--out-dir", targetDir,
		}

		if opts.Sources != "" {
			baseArgs = append(baseArgs, "--sources", opts.Sources)
		}
		if opts.SkipVerify {
			baseArgs = append(baseArgs, "--skip-verify")
		}
		if opts.SkipMetadata {
			baseArgs = append(baseArgs, "--skip-metadata")
		}
		if opts.Verbose {
			baseArgs = append(baseArgs, "--verbose")
		}

		var lastCandidate *progress.CandidateInfo
		var lastResult *progress.ResultInfo
		var lastError string

		var cmdErr error
		var stdoutBuf, stderrBuf bytes.Buffer

		if isAgent {
			// Agent mode: preserve standard machine-readable output format
			cmd := cmdutil.NewCommand(ctx, fetchTrackCmdName, baseArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmdErr = cmd.Run()
		} else {
			// Interactive / non-agent mode (agent=0): stream progress over socket server
			sockServer, sockTarget, sockErr := progress.StartSocketServer(ctx, func(ev progress.Event) {
				switch ev.Type {
				case progress.EventPhaseStart:
					if !opts.Verbose {
						switch ev.Phase {
						case "search":
							fmt.Printf("  • Searching sources: %s\n", opts.Sources)
						case "download":
							fmt.Println("  • Downloading audio stream & artwork...")
						case "verify":
							fmt.Println("  • Inspecting audio quality & spectrum...")
						case "metadata":
							fmt.Println("  • Enriching metadata & cover art...")
						}
					}
				case progress.EventCandidateSelected:
					lastCandidate = ev.Candidate
					if !opts.Verbose && ev.Candidate != nil {
						durStr := ""
						if ev.Candidate.Duration > 0 {
							durStr = fmt.Sprintf(" [%s %s]", ev.Candidate.Source, formatDuration(ev.Candidate.Duration))
						}
						fmt.Printf("  • Selected: %q%s\n", ev.Candidate.Title, durStr)
					}
				case progress.EventComplete:
					lastResult = ev.Result
				case progress.EventError:
					lastError = ev.Error
				}

				if opts.ProgressReporter != nil {
					_ = opts.ProgressReporter.Emit(ev)
				}
			})

			args := append([]string{}, baseArgs...)
			if sockErr == nil && sockServer != nil {
				args = append(args, "--progress-target", sockTarget)
			}

			cmd := cmdutil.NewCommand(ctx, fetchTrackCmdName, args...)
			if opts.Verbose || sockServer == nil {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				cmd.Stdout = &stdoutBuf
				cmd.Stderr = &stderrBuf
			}

			cmdErr = cmd.Run()

			if sockServer != nil {
				_ = sockServer.Close()
			}
		}

		if cmdErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			errMsg := lastError
			if errMsg == "" {
				if stderrBuf.Len() > 0 {
					errMsg = strings.TrimSpace(stderrBuf.String())
				} else {
					errMsg = cmdErr.Error()
				}
			}

			fmt.Printf("  ✗ Failed to download track %q: %s\n", searchTarget, errMsg)
			failureCount++

			if manifest != nil && i < len(manifest.Tracks) {
				manifest.Tracks[i].Status = "failed"
				manifest.Tracks[i].ErrorMessage = errMsg
				_ = SaveManifest(manifest)
			}
		} else {
			// Determine final downloaded file name
			var newFile string
			if lastResult != nil && lastResult.Path != "" {
				newFile = filepath.Base(lastResult.Path)
			}
			if newFile == "" {
				newFile = DetectNewFile(targetDir, preSnapshot)
			}

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

			if !isAgent {
				if lastResult != nil && lastResult.BandwidthRating != "" {
					gainStr := ""
					if lastResult.SuggestedGainDb != 0 {
						gainSign := ""
						if lastResult.SuggestedGainDb > 0 {
							gainSign = "+"
						}
						gainStr = fmt.Sprintf(" | Gain Offset: %s%.1f dB", gainSign, lastResult.SuggestedGainDb)
					}
					fmt.Printf("  • Quality: %s (%d kHz)%s\n", lastResult.BandwidthRating, lastResult.BandwidthHz/1000, gainStr)
				}
				if lastResult != nil && lastResult.Title != "" && lastResult.Album != "" {
					fmt.Printf("  • Metadata: %q (%s, %s)\n", lastResult.Title, lastResult.Album, lastResult.ReleaseYear)
				}
				fmt.Printf("  ✓ Completed: %s\n", finalFilename)
			}

			successCount++

			if manifest != nil && i < len(manifest.Tracks) {
				manifest.Tracks[i].Status = "completed"
				manifest.Tracks[i].ActualFile = finalFilename
				if lastResult != nil {
					manifest.Tracks[i].Duration = lastResult.Duration
					manifest.Tracks[i].BandwidthHz = lastResult.BandwidthHz
					manifest.Tracks[i].QualityRating = lastResult.BandwidthRating
					manifest.Tracks[i].GainOffsetDb = lastResult.SuggestedGainDb
				}
				if lastCandidate != nil {
					manifest.Tracks[i].SourceURL = lastCandidate.WebpageURL
				}
				_ = SaveManifest(manifest)
			}
		}

		if i < totalTracks-1 && trackDownloadDelay > 0 {
			delay := trackDownloadDelay + time.Duration(rand.Intn(1500))*time.Millisecond
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
