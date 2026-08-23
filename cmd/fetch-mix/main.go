package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/downloader"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/llm"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/parser"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/progress"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/scraper"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/search"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/ui"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/youtube"
)

var (
	version = "dev"

	outDir       string
	dryRun       bool
	noCache      bool
	sourcesFlag  string
	skipVerify   bool
	skipMetadata bool
	interactive  bool
	verbose        bool
	llmProvider    string
	llmModel       string
	progressTarget string
	progressSocket string
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Handle second Ctrl+C for forced immediate exit
	go func() {
		<-ctx.Done()
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nForced termination requested. Exiting.")
		os.Exit(130)
	}()

	rootCmd := &cobra.Command{
		Use:          "fetch-mix <set_title_or_url>",
		Short:        "Fetch, extract, and download entire DJ mix tracklists using fetch-track CLI",
		Version:      version,
		SilenceUsage: true,
		Long: `fetch-mix is a CLI tool for DJ sets and mix tracklists.
It searches MixesDB, crawlable web mirrors, 1001tracklists, or YouTube comments (via multi-provider LLMs),
parses tracklists deterministically, and downloads individual tracks using fetch-track CLI.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter DJ set title or URL (e.g. 'Bicep Essential Mix 2014'): ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "" {
					return fmt.Errorf("no query provided")
				}
				args = []string{input}
			}

			targetQuery := strings.TrimSpace(strings.Join(args, " "))

			// Check if target is a YouTube URL or video ID
			videoID := youtube.ExtractVideoID(targetQuery)
			if videoID != "" && (strings.Contains(targetQuery, "youtube.com") || strings.Contains(targetQuery, "youtu.be") || len(targetQuery) == 11) {
				return runYouTubePipeline(cmd.Context(), targetQuery)
			}

			return runMixPipeline(cmd.Context(), targetQuery)
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.Flags().StringVarP(&outDir, "out-dir", "o", "", "Output directory for downloaded tracks (default: cwd/{mix-title})")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview tracklist and download plan without downloading files")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable local caching for search queries, tracklists, and comments")
	rootCmd.Flags().StringVarP(&sourcesFlag, "sources", "s", "youtube,soundcloud", "Comma-separated search sources passed to fetch-track CLI")
	rootCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip DJ audio quality verification in fetch-track CLI")
	rootCmd.Flags().BoolVar(&skipMetadata, "skip-metadata", false, "Skip metadata lookup and cover art tagging in fetch-track CLI")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively choose set search result")
	rootCmd.Flags().StringVarP(&llmProvider, "llm-provider", "p", "auto", "LLM provider name (auto, ollama, litellm, gemini, openai, anthropic, openrouter, deepseek, groq, custom)")
	rootCmd.Flags().StringVarP(&llmModel, "llm-model", "m", "", "LLM model name override")
	rootCmd.Flags().StringVar(&progressTarget, "progress-target", "", "Target URI/address for streaming JSON progress events (e.g. unix:///path/to.sock, tcp://127.0.0.1:9099, fd://3, stdout, stderr)")
	rootCmd.Flags().StringVar(&progressSocket, "progress-socket", "", "Shorthand alias for --progress-target")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose log output")

	youtubeCmd := &cobra.Command{
		Use:          "youtube <youtube_url_or_video_id>",
		Aliases:      []string{"yt"},
		Short:        "Extract tracklist from YouTube comments via LLM and download tracks",
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter YouTube video URL: ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "" {
					return fmt.Errorf("no YouTube URL provided")
				}
				args = []string{input}
			}

			videoURL := strings.TrimSpace(strings.Join(args, " "))
			return runYouTubePipeline(cmd.Context(), videoURL)
		},
	}

	youtubeCmd.Flags().StringVarP(&outDir, "out-dir", "o", "", "Output directory for downloaded tracks (default: cwd/{mix-title})")
	youtubeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview tracklist and download plan without downloading files")
	youtubeCmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable local caching for YouTube comment fetching and tracklist extraction")
	youtubeCmd.Flags().StringVarP(&sourcesFlag, "sources", "s", "youtube,soundcloud", "Comma-separated search sources passed to fetch-track CLI")
	youtubeCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip DJ audio quality verification in fetch-track CLI")
	youtubeCmd.Flags().BoolVar(&skipMetadata, "skip-metadata", false, "Skip metadata lookup and cover art tagging in fetch-track CLI")
	youtubeCmd.Flags().StringVarP(&llmProvider, "llm-provider", "p", "auto", "LLM provider name (auto, ollama, litellm, gemini, openai, anthropic, openrouter, deepseek, groq, custom)")
	youtubeCmd.Flags().StringVarP(&llmModel, "llm-model", "m", "", "LLM model name override")
	youtubeCmd.Flags().StringVar(&progressTarget, "progress-target", "", "Target URI/address for streaming JSON progress events (e.g. unix:///path/to.sock, tcp://127.0.0.1:9099, fd://3, stdout, stderr)")
	youtubeCmd.Flags().StringVar(&progressSocket, "progress-socket", "", "Shorthand alias for --progress-target")

	aiCmd := &cobra.Command{
		Use:          "ai",
		Aliases:      []string{"providers", "models"},
		Short:        "Print supported LLM providers, models, active API key status, and AI configuration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			statuses := llm.GetProviderStatuses()
			sep := ui.Separator("=", 50)

			fmt.Println(sep)
			fmt.Println("Supported LLM Providers & Models (auto-detection priority order):")
			fmt.Println(sep)
			fmt.Printf("%-14s %-26s %-20s %s\n", "PROVIDER", "DEFAULT MODEL", "ENV VAR", "STATUS")
			fmt.Println(ui.Separator("-", 50))

			for _, s := range statuses {
				fmt.Printf("%-14s %-26s %-20s %s\n", s.ID, s.DefaultModel, s.EnvVar, s.Status)
			}

			fmt.Println(sep)
			fmt.Println("Usage Examples:")
			fmt.Println("  fetch-mix youtube -p auto <url>")
			fmt.Println("  fetch-mix youtube -p litellm -m gpt-4o-mini <url>")
			fmt.Println("  fetch-mix youtube -p ollama -m llama3.2 <url>")
			fmt.Println("  fetch-mix youtube -p openai -m gpt-4o-mini <url>")
			fmt.Println("  fetch-mix youtube -p anthropic -m claude-3-5-haiku-latest <url>")
			fmt.Println(sep)
			return nil
		},
	}

	depsCmd := &cobra.Command{
		Use:          "dependencies",
		Aliases:      []string{"deps"},
		Short:        "Verify required external binary dependencies (fetch-track, yt-dlp, ffmpeg, firecrawl)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			isAgent := deps.IsAgentMode()
			reports, err := deps.VerifyDependencies(cmd.Context())

			if isAgent {
				for _, r := range reports {
					if r.Satisfied {
						fmt.Printf("%s: ok (version %s, min %s)\n", r.Name, r.DetectedVersion, r.MinVersion)
					} else if !r.Installed {
						fmt.Printf("%s: missing\n", r.Name)
					} else {
						fmt.Printf("%s: fail (version %s, min %s)\n", r.Name, r.DetectedVersion, r.MinVersion)
					}
				}
				if err != nil {
					fmt.Printf("status: error\nerror: %v\n", err)
					return err
				}
				fmt.Println("status: ok")
				return nil
			}

			for _, r := range reports {
				if r.Satisfied {
					fmt.Printf("%s: %s (min %s) [OK]\n", r.Name, r.DetectedVersion, r.MinVersion)
				} else if !r.Installed {
					fmt.Printf("%s: missing in $PATH [FAIL] - %s\n", r.Name, r.Error)
				} else {
					fmt.Printf("%s: %s (min %s) [FAIL] - %s\n", r.Name, r.DetectedVersion, r.MinVersion, r.Error)
				}
			}

			if err != nil {
				return err
			}
			fmt.Println("\nAll required dependencies are installed and operational.")
			return nil
		},
	}

	rootCmd.AddCommand(youtubeCmd)
	rootCmd.AddCommand(aiCmd)
	rootCmd.AddCommand(depsCmd)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "Operation canceled by user.")
			os.Exit(130)
		}
		os.Exit(1)
	}
}

func resolveProgressReporter(ctx context.Context) (*progress.Reporter, error) {
	targetURI := progressTarget
	if targetURI == "" {
		targetURI = progressSocket
	}
	if targetURI == "" {
		targetURI = os.Getenv("FETCH_MIX_PROGRESS_TARGET")
	}
	if strings.TrimSpace(targetURI) == "" {
		return nil, nil
	}
	return progress.NewReporter(ctx, targetURI)
}

func runMixPipeline(ctx context.Context, query string) error {
	var chosenSet types.SearchResult

	isDirectCrawlable := (strings.HasPrefix(query, "http://") || strings.HasPrefix(query, "https://")) &&
		!strings.Contains(strings.ToLower(query), "1001tracklists.com")

	if isDirectCrawlable {
		fmt.Println("Direct crawlable URL detected. Bypassing search...")
		pageTitle := "Direct URL Tracklist"
		parts := strings.Split(query, "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			slug := parts[len(parts)-1]
			slug = strings.TrimSuffix(slug, ".html")
			slug = strings.TrimSuffix(slug, ".htm")
			slug = strings.ReplaceAll(slug, "-", " ")
			slug = strings.ReplaceAll(slug, "_", " ")
			pageTitle = strings.Title(slug)
		}
		chosenSet = types.SearchResult{
			Title: pageTitle,
			URL:   query,
		}
	} else {
		fmt.Printf("Searching for %q...\n", query)
		results, err := search.SearchSet(ctx, query)
		if err != nil {
			return fmt.Errorf("search error: %w", err)
		}

		if len(results) == 0 {
			return fmt.Errorf("no matching DJ sets found for %q", query)
		}

		if len(results) == 1 {
			chosenSet = results[0]
			fmt.Printf("Found set: %q\n", chosenSet.Title)
		} else {
			fmt.Println("\nMultiple sets found. Please choose one:")
			for i, r := range results {
				fmt.Printf("  [%d] %s\n", i+1, r.Title)
			}

			if interactive && !deps.IsAgentMode() {
				reader := bufio.NewReader(os.Stdin)
				fmt.Printf("Choose option (1-%d) [1]: ", len(results))
				choiceStr, _ := reader.ReadString('\n')
				choiceStr = strings.TrimSpace(choiceStr)
				choiceNum, err := strconv.Atoi(choiceStr)
				if err != nil || choiceNum < 1 || choiceNum > len(results) {
					chosenSet = results[0]
				} else {
					chosenSet = results[choiceNum-1]
				}
			} else {
				chosenSet = results[0]
			}
			fmt.Printf("Selected: %q\n", chosenSet.Title)
		}
	}

	fmt.Printf("\nScraping tracklist from: %s...\n", chosenSet.URL)
	markdown, err := scraper.ScrapeSetWithCache(ctx, chosenSet.URL, noCache)
	if err != nil {
		return fmt.Errorf("scraping error: %w", err)
	}

	tracks, skippedItems := parser.ParseTracklist(markdown)
	if len(tracks) == 0 {
		return fmt.Errorf("could not extract any tracks from the tracklist page")
	}

	if !dryRun {
		hasHours := downloader.HasHourTimestamps(tracks, skippedItems)
		fmt.Printf("\nFound %d tracks:\n", len(tracks))
		for _, tr := range tracks {
			tsStr := downloader.NormalizeTimestamp(tr.Timestamp, hasHours)
			if tsStr != "" {
				fmt.Printf("  [%s] %s - %s\n", tsStr, tr.Artist, tr.Title)
			} else {
				fmt.Printf("  %s - %s\n", tr.Artist, tr.Title)
			}
		}
	}

	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}

	opts := downloader.DownloadOptions{
		MixTitle:         chosenSet.Title,
		Tracks:           tracks,
		SkippedItems:     skippedItems,
		OutputDir:        outDir,
		DryRun:           dryRun,
		Sources:          sourcesFlag,
		SkipVerify:       skipVerify,
		SkipMetadata:     skipMetadata,
		Verbose:          verbose,
		ProgressReporter: reporter,
	}

	return downloader.DownloadSet(ctx, opts)
}

func runYouTubePipeline(ctx context.Context, videoURL string) error {
	fmt.Printf("Processing YouTube video tracklist comments: %s...\n", videoURL)
	tracks, skippedItems, videoTitle, err := youtube.ProcessYouTubeComments(ctx, videoURL, noCache, llmProvider, llmModel)
	if err != nil {
		return fmt.Errorf("YouTube comments extraction failed: %w", err)
	}

	if !dryRun {
		hasHours := downloader.HasHourTimestamps(tracks, skippedItems)
		fmt.Printf("\nFound tracklist for: %q\n", videoTitle)
		fmt.Printf("Extracted %d tracks:\n", len(tracks))
		for _, tr := range tracks {
			tsStr := downloader.NormalizeTimestamp(tr.Timestamp, hasHours)
			if tsStr != "" {
				fmt.Printf("  [%s] %s - %s\n", tsStr, tr.Artist, tr.Title)
			} else {
				fmt.Printf("  %s - %s\n", tr.Artist, tr.Title)
			}
		}
	}

	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}

	opts := downloader.DownloadOptions{
		MixTitle:         videoTitle,
		Tracks:           tracks,
		SkippedItems:     skippedItems,
		OutputDir:        outDir,
		DryRun:           dryRun,
		Sources:          sourcesFlag,
		SkipVerify:       skipVerify,
		SkipMetadata:     skipMetadata,
		Verbose:          verbose,
		ProgressReporter: reporter,
	}

	return downloader.DownloadSet(ctx, opts)
}
