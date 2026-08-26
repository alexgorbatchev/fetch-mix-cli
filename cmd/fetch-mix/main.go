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
	"github.com/spf13/pflag"

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
	"github.com/alexgorbatchev/godeps"
)

var (
	version = "dev"

	outDir         string
	dryRun         bool
	noCache        bool
	sourcesFlag    string
	skipVerify     bool
	skipMetadata   bool
	interactive    bool
	verbose        bool
	llmProvider    string
	llmModel       string
	progressTarget string
	progressSocket string
	autoInstall    bool
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

	if code := runMain(ctx); code != 0 {
		os.Exit(code)
	}
}

func runMain(ctx context.Context) int {
	if err := execute(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "Operation canceled by user.")
			return 130
		}
		return 1
	}
	return 0
}

func execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
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
				return cmd.Help()
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

	addPipelineFlags(rootCmd.Flags())
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively choose search result")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(newYouTubeCmd())
	rootCmd.AddCommand(newAICmd())
	rootCmd.AddCommand(newDepsCmd())
	rootCmd.AddCommand(newUpgradeCmd())

	setupHelp(rootCmd)

	return rootCmd
}

func addPipelineFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&outDir, "out-dir", "o", "", "Output directory for tracks (default: cwd/{mix-title})")
	flags.BoolVar(&dryRun, "dry-run", false, "Preview tracklist without downloading")
	flags.BoolVar(&noCache, "no-cache", false, "Disable local caching")
	flags.StringVarP(&sourcesFlag, "sources", "s", "youtube,soundcloud", "Search sources passed to fetch-track CLI")
	flags.BoolVar(&skipVerify, "skip-verify", false, "Skip audio quality verification")
	flags.BoolVar(&skipMetadata, "skip-metadata", false, "Skip cover art and metadata tagging")
	flags.StringVarP(&llmProvider, "llm-provider", "p", "auto", "LLM provider name (e.g. ollama, gemini, openai)")
	flags.StringVarP(&llmModel, "llm-model", "m", "", "LLM model override")
	flags.StringVar(&progressTarget, "progress-target", "", "Target URI for streaming JSON progress events")
	flags.StringVar(&progressSocket, "progress-socket", "", "Shorthand alias for --progress-target")
	flags.BoolVar(&autoInstall, "auto-install", false, "Auto-install missing dependencies")
}

func newYouTubeCmd() *cobra.Command {
	youtubeCmd := &cobra.Command{
		Use:          "youtube <url|id>",
		Aliases:      []string{"yt"},
		Short:        "Extract tracklist from YouTube comments and download",
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			videoURL := strings.TrimSpace(strings.Join(args, " "))
			return runYouTubePipeline(cmd.Context(), videoURL)
		},
	}

	addPipelineFlags(youtubeCmd.Flags())
	return youtubeCmd
}

func newAICmd() *cobra.Command {
	aiCmd := &cobra.Command{
		Use:          "ai",
		Aliases:      []string{"providers", "models"},
		Short:        "Manage and inspect AI / LLM configuration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIList(cmd)
		},
	}

	aiListCmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List supported LLM providers and models",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIList(cmd)
		},
	}

	aiInspectCmd := &cobra.Command{
		Use:          "inspect <provider>",
		Aliases:      []string{"show", "get"},
		Short:        "Inspect configuration and status of an LLM provider",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIInspect(cmd, args[0])
		},
	}

	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiInspectCmd)
	return aiCmd
}

func runAIList(cmd *cobra.Command) error {
	statuses := llm.GetProviderStatuses()
	if deps.IsAgentMode() {
		for _, s := range statuses {
			fmt.Printf("%s\t%s\t%s\t%s\n", s.ID, s.DefaultModel, s.EnvVar, s.Status)
		}
		return nil
	}

	sep := ui.Separator("=", 50)
	if sep != "" {
		fmt.Println(sep)
	}
	fmt.Println("Supported LLM Providers & Models (auto-detection priority order):")
	if sep != "" {
		fmt.Println(sep)
	}
	fmt.Printf("%-14s %-26s %-20s %s\n", "PROVIDER", "DEFAULT MODEL", "ENV VAR", "STATUS")
	if subSep := ui.Separator("-", 50); subSep != "" {
		fmt.Println(subSep)
	}

	for _, s := range statuses {
		fmt.Printf("%-14s %-26s %-20s %s\n", s.ID, s.DefaultModel, s.EnvVar, s.Status)
	}

	if sep != "" {
		fmt.Println(sep)
	}
	fmt.Println("Usage Examples:")
	fmt.Println("  fetch-mix youtube -p auto <url>")
	fmt.Println("  fetch-mix youtube -p litellm -m gpt-4o-mini <url>")
	fmt.Println("  fetch-mix youtube -p ollama -m llama3.2 <url>")
	fmt.Println("  fetch-mix youtube -p openai -m gpt-4o-mini <url>")
	fmt.Println("  fetch-mix youtube -p anthropic -m claude-3-5-haiku-latest <url>")
	if sep != "" {
		fmt.Println(sep)
	}
	return nil
}

func runAIInspect(cmd *cobra.Command, providerName string) error {
	providerID := strings.ToLower(strings.TrimSpace(providerName))
	statuses := llm.GetProviderStatuses()
	var found *llm.ProviderInfo
	for _, s := range statuses {
		if strings.ToLower(s.ID) == providerID || strings.ToLower(s.Name) == providerID {
			target := s
			found = &target
			break
		}
	}

	if found == nil {
		var validIDs []string
		for _, s := range statuses {
			validIDs = append(validIDs, s.ID)
		}
		return fmt.Errorf("unknown provider %q (supported: %s)", providerName, strings.Join(validIDs, ", "))
	}

	if deps.IsAgentMode() {
		fmt.Printf("provider: %s\nname: %s\ndefault_model: %s\nenv_var: %s\nstatus: %s\n",
			found.ID, found.Name, found.DefaultModel, found.EnvVar, found.Status)
		return nil
	}

	sep := ui.Separator("=", 50)
	if sep != "" {
		fmt.Println(sep)
	}
	fmt.Printf("Provider Details: %s\n", found.Name)
	if sep != "" {
		fmt.Println(sep)
	}
	fmt.Printf("Provider ID    : %s\n", found.ID)
	fmt.Printf("Default Model  : %s\n", found.DefaultModel)
	fmt.Printf("Environment Var: %s\n", found.EnvVar)
	fmt.Printf("Status         : %s\n", found.Status)
	if sep != "" {
		fmt.Println(sep)
	}
	return nil
}

func newDepsCmd() *cobra.Command {
	depsCmd := &cobra.Command{
		Use:          "dependencies",
		Aliases:      []string{"deps"},
		Short:        "Manage external binary dependencies (fetch-track, yt-dlp, ffmpeg)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDepsVerify(cmd)
		},
	}

	depsVerifyCmd := &cobra.Command{
		Use:          "verify",
		Aliases:      []string{"check", "status"},
		Short:        "Verify required external binary dependencies",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDepsVerify(cmd)
		},
	}

	depsInstallCmd := &cobra.Command{
		Use:          "install [dep...]",
		Aliases:      []string{"add", "get"},
		Short:        "Install missing external dependencies",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDepsInstall(cmd, args)
		},
	}

	depsUpdateCmd := &cobra.Command{
		Use:          "update [dep...]",
		Aliases:      []string{"upgrade"},
		Short:        "Update external dependencies to latest versions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDepsUpdate(cmd, args)
		},
	}

	depsCmd.AddCommand(depsVerifyCmd)
	depsCmd.AddCommand(depsInstallCmd)
	depsCmd.AddCommand(depsUpdateCmd)
	return depsCmd
}

func runDepsVerify(cmd *cobra.Command) error {
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
}

func runDepsInstall(cmd *cobra.Command, args []string) error {
	_ = deps.InitManagedPath()
	isAgent := deps.IsAgentMode()
	if len(args) > 0 {
		for _, depName := range args {
			if !isAgent {
				fmt.Printf("Installing %s...\n", depName)
			}
			if err := deps.InstallDependency(cmd.Context(), depName); err != nil {
				return fmt.Errorf("installing %s: %w", depName, err)
			}
			if isAgent {
				fmt.Printf("OK: installed %s\n", depName)
			} else {
				fmt.Printf("[OK] %s installed successfully.\n", depName)
			}
		}
		return nil
	}

	if !isAgent {
		fmt.Println("Checking and installing missing dependencies...")
	}
	installed, err := deps.InstallMissingDependencies(cmd.Context())
	if err != nil {
		return err
	}
	if len(installed) == 0 {
		if isAgent {
			fmt.Println("status: satisfied")
		} else {
			fmt.Println("All dependencies are already satisfied.")
		}
	} else {
		if isAgent {
			for _, depName := range installed {
				fmt.Printf("OK: installed %s\n", depName)
			}
		} else {
			fmt.Printf("[OK] Successfully installed: %s\n", strings.Join(installed, ", "))
		}
	}
	return nil
}

func runDepsUpdate(cmd *cobra.Command, args []string) error {
	_ = deps.InitManagedPath()
	isAgent := deps.IsAgentMode()
	if len(args) > 0 {
		for _, depName := range args {
			if !isAgent {
				fmt.Printf("Updating %s...\n", depName)
			}
			if err := deps.UpdateDependency(cmd.Context(), depName); err != nil {
				return fmt.Errorf("updating %s: %w", depName, err)
			}
			if isAgent {
				fmt.Printf("OK: updated %s\n", depName)
			} else {
				fmt.Printf("[OK] %s updated successfully.\n", depName)
			}
		}
		return nil
	}

	if !isAgent {
		fmt.Println("Updating all dependencies to latest versions...")
	}
	updated, err := deps.UpdateAllDependencies(cmd.Context())
	if err != nil {
		return err
	}
	if isAgent {
		for _, depName := range updated {
			fmt.Printf("OK: updated %s\n", depName)
		}
	} else {
		fmt.Printf("[OK] Successfully updated: %s\n", strings.Join(updated, ", "))
	}
	return nil
}

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "upgrade",
		Aliases:      []string{"self-update", "update-self"},
		Short:        "Upgrade fetch-mix binary to latest release",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			isAgent := deps.IsAgentMode()
			if !isAgent {
				fmt.Printf("Checking for newer fetch-mix release (current version: %s)...\n", version)
			}
			newVer, err := godeps.UpgradeSelf(cmd.Context(), "alexgorbatchev", "fetch-mix-cli", version)
			if err != nil {
				if strings.Contains(err.Error(), "already at the latest version") {
					if isAgent {
						fmt.Printf("status: current\nversion: %s\n", version)
					} else {
						fmt.Printf("fetch-mix is already up to date (%s).\n", version)
					}
					return nil
				}
				return fmt.Errorf("upgrade failed: %w", err)
			}
			if isAgent {
				fmt.Printf("OK: upgraded fetch-mix to %s\n", newVer)
			} else {
				fmt.Printf("[OK] Successfully upgraded fetch-mix to version %s!\n", newVer)
			}
			return nil
		},
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

func ensureDependencies(ctx context.Context) error {
	return deps.NewManager().Ensure(ctx, godeps.EnsureOptions{
		AutoInstall: autoInstall,
	})
}

func runMixPipeline(ctx context.Context, query string) error {
	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}

	if !dryRun {
		if err := ensureDependencies(ctx); err != nil {
			return err
		}
	}

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
		if reporter != nil {
			_ = reporter.Emit(progress.Event{
				Type:    progress.EventPhaseStart,
				Phase:   "search",
				Message: fmt.Sprintf("Searching for %q...", query),
			})
		}
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

	if reporter != nil {
		_ = reporter.Emit(progress.Event{
			Type:    progress.EventPhaseStart,
			Phase:   "scrape",
			Message: fmt.Sprintf("Scraping tracklist from %s", chosenSet.URL),
		})
	}
	fmt.Printf("\nScraping tracklist from: %s...\n", chosenSet.URL)
	markdown, err := scraper.ScrapeSetWithCache(ctx, chosenSet.URL, noCache)
	if err != nil {
		return fmt.Errorf("scraping error: %w", err)
	}

	tracks, skippedItems, err := parser.ParseTracklistWithAI(ctx, markdown, chosenSet.Title, llmProvider, llmModel)
	if err != nil || len(tracks) == 0 {
		if err != nil {
			return fmt.Errorf("could not extract tracks from tracklist page: %w", err)
		}
		return fmt.Errorf("could not extract any tracks from the tracklist page")
	}

	if reporter != nil {
		_ = reporter.Emit(progress.Event{
			Type:    progress.EventProgress,
			Phase:   "parse",
			Message: fmt.Sprintf("Extracted %d tracks from tracklist", len(tracks)),
		})
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
	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}

	if !dryRun {
		if err := ensureDependencies(ctx); err != nil {
			return err
		}
	}

	if reporter != nil {
		_ = reporter.Emit(progress.Event{
			Type:    progress.EventPhaseStart,
			Phase:   "youtube_comments",
			Message: fmt.Sprintf("Processing YouTube video comments for %s", videoURL),
		})
	}

	fmt.Printf("Processing YouTube video tracklist comments: %s...\n", videoURL)
	tracks, skippedItems, videoTitle, err := youtube.ProcessYouTubeComments(ctx, videoURL, noCache, llmProvider, llmModel)
	if err != nil {
		return fmt.Errorf("YouTube comments extraction failed: %w", err)
	}

	if reporter != nil {
		_ = reporter.Emit(progress.Event{
			Type:    progress.EventProgress,
			Phase:   "parse",
			Message: fmt.Sprintf("Extracted %d tracks from YouTube comments", len(tracks)),
		})
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
