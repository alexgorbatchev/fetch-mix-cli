package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/display"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
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
		display.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

var jsonOutput bool
var listCandidates bool
var candidateURL string
var resultWriter io.Writer = os.Stdout

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "fetch-mix <url|query>",
		Short:         "Fetch, extract, and download entire DJ mix tracklists using fetch-track CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `fetch-mix is a CLI tool for DJ sets and mix tracklists.
It searches MixesDB, crawlable web mirrors, 1001tracklists, or YouTube comments (via multi-provider LLMs),
parses tracklists deterministically, and downloads individual tracks using fetch-track CLI.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if jsonOutput {
					return writeJSONFailure(fmt.Errorf("a target is required"))
				}
				return cmd.Help()
			}

			targetQuery := strings.TrimSpace(strings.Join(args, " "))
			if candidateURL != "" {
				if err := validateCandidateURL(candidateURL); err != nil {
					return writeJSONFailure(err)
				}
				targetQuery = candidateURL
			}

			// Check if target is a YouTube URL or video ID
			videoID := youtube.ExtractVideoID(targetQuery)
			if videoID != "" && (strings.Contains(targetQuery, "youtube.com") || strings.Contains(targetQuery, "youtu.be") || len(targetQuery) == 11) {
				return runYouTubePipeline(cmd.Context(), targetQuery)
			}

			return runMixPipeline(cmd.Context(), targetQuery)
		},
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output structured results as JSON")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) { resultWriter = cmd.OutOrStdout() }
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	addPipelineFlags(rootCmd.Flags())
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively choose search result")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(newYouTubeCmd())
	rootCmd.AddCommand(newAICmd())
	rootCmd.AddCommand(newDepsCmd())
	selfCmd := &cobra.Command{Use: "self", Short: "Manage this executable"}
	selfCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(selfCmd)

	setupHelp(rootCmd)

	return rootCmd
}

func addPipelineFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&listCandidates, "list-candidates", false, "List matching sets without extracting or downloading")
	flags.StringVar(&candidateURL, "candidate-url", "", "Use this exact selected source URL without searching again")
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
				if jsonOutput {
					return writeJSONFailure(fmt.Errorf("a target is required"))
				}
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
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if jsonOutput {
					return writeJSONFailure(fmt.Errorf("a target is required"))
				}
				return cmd.Help()
			}
			return runAIInspect(cmd, args[0])
		},
	}

	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiInspectCmd)
	return aiCmd
}

func runAIList(cmd *cobra.Command) error {
	statuses := llm.GetProviderStatuses()
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Providers []llm.ProviderInfo `json:"providers"`
		}{statuses})
	}
	if godeps.IsAgentMode() {
		for _, s := range statuses {
			display.Fprintf(diagnosticWriter(), "%s\t%s\t%s\t%s\n", s.ID, s.DefaultModel, s.EnvVar, s.Status)
		}
		return nil
	}

	sep := ui.Separator("=", 50)
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
	}
	fmt.Fprintln(diagnosticWriter(), "Supported LLM Providers & Models (auto-detection priority order):")
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
	}
	for _, s := range statuses {
		display.Fprintf(diagnosticWriter(), "%s (%s)\n  Model: %s\n  Environment: %s\n", s.ID, s.Status, s.DefaultModel, s.EnvVar)
	}

	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
	}
	fmt.Fprintln(diagnosticWriter(), "Usage Examples:")
	fmt.Fprintln(diagnosticWriter(), "  fetch-mix youtube -p auto <url>")
	fmt.Fprintln(diagnosticWriter(), "  fetch-mix youtube -p litellm -m gpt-4o-mini <url>")
	fmt.Fprintln(diagnosticWriter(), "  fetch-mix youtube -p ollama -m llama3.2 <url>")
	fmt.Fprintln(diagnosticWriter(), "  fetch-mix youtube -p openai -m gpt-4o-mini <url>")
	fmt.Fprintln(diagnosticWriter(), "  fetch-mix youtube -p anthropic -m claude-3-5-haiku-latest <url>")
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
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
		return writeJSONFailure(fmt.Errorf("unknown provider %q (supported: %s)", providerName, strings.Join(validIDs, ", ")))
	}

	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(found)
	}
	if godeps.IsAgentMode() {
		display.Fprintf(diagnosticWriter(), "provider: %s\nname: %s\ndefault_model: %s\nenv_var: %s\nstatus: %s\n",
			found.ID, found.Name, found.DefaultModel, found.EnvVar, found.Status)
		return nil
	}

	sep := ui.Separator("=", 50)
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
	}
	display.Fprintf(diagnosticWriter(), "Provider Details: %s\n", found.Name)
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
	}
	display.Fprintf(diagnosticWriter(), "Provider ID    : %s\n", found.ID)
	display.Fprintf(diagnosticWriter(), "Default Model  : %s\n", found.DefaultModel)
	display.Fprintf(diagnosticWriter(), "Environment Var: %s\n", found.EnvVar)
	display.Fprintf(diagnosticWriter(), "Status         : %s\n", found.Status)
	if sep != "" {
		fmt.Fprintln(diagnosticWriter(), sep)
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
	if jsonOutput {
		return runDepsJSON(cmd, nil, "inspect")
	}
	isAgent := godeps.IsAgentMode()
	reports, err := deps.VerifyDependencies(cmd.Context())

	if isAgent {
		for _, r := range reports {
			if r.Satisfied {
				display.Fprintf(diagnosticWriter(), "%s: ok (version %s, min %s)\n", r.Name, r.DetectedVersion, r.MinVersion)
			} else if !r.Installed {
				display.Fprintf(diagnosticWriter(), "%s: missing\n", r.Name)
			} else {
				display.Fprintf(diagnosticWriter(), "%s: fail (version %s, min %s)\n", r.Name, r.DetectedVersion, r.MinVersion)
			}
		}
		if err != nil {
			display.Fprintf(diagnosticWriter(), "status: error\nerror: %v\n", err)
			return err
		}
		fmt.Fprintln(diagnosticWriter(), "status: ok")
		return nil
	}

	for _, r := range reports {
		if r.Satisfied {
			display.Fprintf(diagnosticWriter(), "%s: %s (min %s) [OK]\n", r.Name, r.DetectedVersion, r.MinVersion)
		} else if !r.Installed {
			display.Fprintf(diagnosticWriter(), "%s: missing in $PATH [FAIL] - %s\n", r.Name, r.Error)
		} else {
			display.Fprintf(diagnosticWriter(), "%s: %s (min %s) [FAIL] - %s\n", r.Name, r.DetectedVersion, r.MinVersion, r.Error)
		}
	}

	if err != nil {
		return err
	}
	fmt.Fprintln(diagnosticWriter(), "\nAll required dependencies are installed and operational.")
	return nil
}

func runDepsInstall(cmd *cobra.Command, args []string) error {
	if jsonOutput {
		return runDepsJSON(cmd, args, "install")
	}
	_ = deps.InitManagedPath()
	isAgent := godeps.IsAgentMode()
	if len(args) > 0 {
		for _, depName := range args {
			if !isAgent {
				display.Fprintf(diagnosticWriter(), "Installing %s...\n", depName)
			}
			if err := deps.InstallDependency(cmd.Context(), depName); err != nil {
				return fmt.Errorf("installing %s: %w", depName, err)
			}
			if isAgent {
				display.Fprintf(diagnosticWriter(), "OK: installed %s\n", depName)
			} else {
				display.Fprintf(diagnosticWriter(), "[OK] %s installed successfully.\n", depName)
			}
		}
		return nil
	}

	if !isAgent {
		fmt.Fprintln(diagnosticWriter(), "Checking and installing missing dependencies...")
	}
	installed, err := deps.InstallMissingDependencies(cmd.Context())
	if err != nil {
		return err
	}
	if len(installed) == 0 {
		if isAgent {
			fmt.Fprintln(diagnosticWriter(), "status: satisfied")
		} else {
			fmt.Fprintln(diagnosticWriter(), "All dependencies are already satisfied.")
		}
	} else {
		if isAgent {
			for _, depName := range installed {
				display.Fprintf(diagnosticWriter(), "OK: installed %s\n", depName)
			}
		} else {
			display.Fprintf(diagnosticWriter(), "[OK] Successfully installed: %s\n", strings.Join(installed, ", "))
		}
	}
	return nil
}

func runDepsUpdate(cmd *cobra.Command, args []string) error {
	if jsonOutput {
		return runDepsJSON(cmd, args, "update")
	}
	_ = deps.InitManagedPath()
	isAgent := godeps.IsAgentMode()
	if len(args) > 0 {
		for _, depName := range args {
			if !isAgent {
				display.Fprintf(diagnosticWriter(), "Updating %s...\n", depName)
			}
			if err := deps.UpdateDependency(cmd.Context(), depName); err != nil {
				return fmt.Errorf("updating %s: %w", depName, err)
			}
			if isAgent {
				display.Fprintf(diagnosticWriter(), "OK: updated %s\n", depName)
			} else {
				display.Fprintf(diagnosticWriter(), "[OK] %s updated successfully.\n", depName)
			}
		}
		return nil
	}

	if !isAgent {
		fmt.Fprintln(diagnosticWriter(), "Updating all dependencies to latest versions...")
	}
	updated, err := deps.UpdateAllDependencies(cmd.Context())
	if err != nil {
		return err
	}
	if isAgent {
		for _, depName := range updated {
			display.Fprintf(diagnosticWriter(), "OK: updated %s\n", depName)
		}
	} else {
		display.Fprintf(diagnosticWriter(), "[OK] Successfully updated: %s\n", strings.Join(updated, ", "))
	}
	return nil
}

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "upgrade",
		Short:        "Upgrade fetch-mix binary to latest release",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runUpgradeJSON(cmd, version)
			}
			isAgent := godeps.IsAgentMode()
			if !isAgent {
				display.Fprintf(diagnosticWriter(), "Checking for newer fetch-mix release (current version: %s)...\n", version)
			}
			newVer, err := godeps.UpgradeSelf(cmd.Context(), "alexgorbatchev", "fetch-mix-cli", version)
			if err != nil {
				if strings.Contains(err.Error(), "already at the latest version") {
					if isAgent {
						display.Fprintf(diagnosticWriter(), "status: current\nversion: %s\n", version)
					} else {
						display.Fprintf(diagnosticWriter(), "fetch-mix is already up to date (%s).\n", version)
					}
					return nil
				}
				return fmt.Errorf("upgrade failed: %w", err)
			}
			if isAgent {
				display.Fprintf(diagnosticWriter(), "OK: upgraded fetch-mix to %s\n", newVer)
			} else {
				display.Fprintf(diagnosticWriter(), "[OK] Successfully upgraded fetch-mix to version %s!\n", newVer)
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

// ensureDependencies verifies required tools. JSON callers are machines, so
// without --auto-install they get the verification failure instead of a prompt.
// Ensure writes its messages to stderr and fails unless installation satisfied
// every dependency.
func ensureDependencies(ctx context.Context) error {
	if jsonOutput && !autoInstall {
		if err := deps.InitManagedPath(); err != nil {
			return err
		}
		_, err := deps.NewManager().Verify(ctx)
		return err
	}
	return deps.NewManager().Ensure(ctx, godeps.EnsureOptions{
		AutoInstall: autoInstall,
	})
}

func runMixPipeline(ctx context.Context, query string) (runErr error) {
	defer func() { runErr = writeJSONFailure(runErr) }()
	if candidateURL != "" {
		if err := validateCandidateURL(candidateURL); err != nil {
			return err
		}
		query = candidateURL
	}
	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}

	if !dryRun && !listCandidates {
		if err := ensureDependencies(ctx); err != nil {
			return err
		}
	}

	var chosenSet types.SearchResult

	isDirectCrawlable := candidateURL != "" || ((strings.HasPrefix(query, "http://") || strings.HasPrefix(query, "https://")) &&
		!strings.Contains(strings.ToLower(query), "1001tracklists.com"))

	if isDirectCrawlable {
		fmt.Fprintln(diagnosticWriter(), "Direct crawlable URL detected. Bypassing search...")
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
		if listCandidates {
			return json.NewEncoder(resultWriter).Encode(mixCandidateResult{Status: "success", Candidates: []types.SearchResult{chosenSet}})
		}
	} else {
		if reporter != nil {
			_ = reporter.Emit(progress.Event{
				Type:    progress.EventPhaseStart,
				Phase:   "search",
				Message: fmt.Sprintf("Searching for %q...", query),
			})
		}
		display.Fprintf(diagnosticWriter(), "Searching for %q...\n", query)
		results, err := search.SearchSet(ctx, query)
		if err != nil {
			return fmt.Errorf("search error: %w", err)
		}

		if listCandidates {
			return json.NewEncoder(resultWriter).Encode(mixCandidateResult{Status: "success", Candidates: results})
		}
		if len(results) == 0 {
			return fmt.Errorf("no matching DJ sets found for %q", query)
		}

		if len(results) == 1 {
			chosenSet = results[0]
			display.Fprintf(diagnosticWriter(), "Found set: %q\n", chosenSet.Title)
		} else {
			if !godeps.IsAgentMode() {
				fmt.Fprintln(diagnosticWriter(), "\nMatching sets:")
			}
			for i, r := range results {
				if godeps.IsAgentMode() {
					display.Fprintf(diagnosticWriter(), "result: %d\t%s\n", i+1, r.Title)
				} else {
					display.Fprintf(diagnosticWriter(), "  [%d] %s\n", i+1, r.Title)
				}
			}

			if interactive && !godeps.IsAgentMode() {
				reader := bufio.NewReader(os.Stdin)
				display.Fprintf(diagnosticWriter(), "Choose option (1-%d) [1]: ", len(results))
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
			display.Fprintf(diagnosticWriter(), "Selected: %q\n", chosenSet.Title)
		}
	}

	if reporter != nil {
		_ = reporter.Emit(progress.Event{
			Type:    progress.EventPhaseStart,
			Phase:   "scrape",
			Message: fmt.Sprintf("Scraping tracklist from %s", chosenSet.URL),
		})
	}
	display.Fprintf(diagnosticWriter(), "\nScraping tracklist from: %s...\n", chosenSet.URL)
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
		display.Fprintf(diagnosticWriter(), "\nFound %d tracks:\n", len(tracks))
		for _, tr := range tracks {
			tsStr := downloader.NormalizeTimestamp(tr.Timestamp, hasHours)
			if tsStr != "" {
				display.Fprintf(diagnosticWriter(), "  [%s] %s - %s\n", tsStr, tr.Artist, tr.Title)
			} else {
				display.Fprintf(diagnosticWriter(), "  %s - %s\n", tr.Artist, tr.Title)
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

	return finishMix(ctx, opts, chosenSet.URL)
}

func runYouTubePipeline(ctx context.Context, videoURL string) (runErr error) {
	defer func() { runErr = writeJSONFailure(runErr) }()
	if candidateURL != "" {
		if err := validateCandidateURL(candidateURL); err != nil {
			return err
		}
		videoURL = candidateURL
	}
	reporter, err := resolveProgressReporter(ctx)
	if err != nil {
		return fmt.Errorf("initializing progress reporter: %w", err)
	}
	if reporter != nil {
		defer reporter.Close()
	}
	if listCandidates {
		id := youtube.ExtractVideoID(videoURL)
		if id == "" {
			return fmt.Errorf("YouTube comments extractor requires a YouTube URL or video ID")
		}
		return json.NewEncoder(resultWriter).Encode(mixCandidateResult{Status: "success", Candidates: []types.SearchResult{{Title: videoURL, URL: "https://www.youtube.com/watch?v=" + id}}})
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

	display.Fprintf(diagnosticWriter(), "Processing YouTube video tracklist comments: %s...\n", videoURL)
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
		display.Fprintf(diagnosticWriter(), "\nFound tracklist for: %q\n", videoTitle)
		display.Fprintf(diagnosticWriter(), "Extracted %d tracks:\n", len(tracks))
		for _, tr := range tracks {
			tsStr := downloader.NormalizeTimestamp(tr.Timestamp, hasHours)
			if tsStr != "" {
				display.Fprintf(diagnosticWriter(), "  [%s] %s - %s\n", tsStr, tr.Artist, tr.Title)
			} else {
				display.Fprintf(diagnosticWriter(), "  %s - %s\n", tr.Artist, tr.Title)
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

	return finishMix(ctx, opts, videoURL)
}

func diagnosticWriter() io.Writer {
	if jsonOutput {
		return os.Stderr
	}
	return os.Stdout
}

func validateCandidateURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("candidate-url must be an absolute HTTP or HTTPS URL")
	}
	return nil
}

type mixCandidateResult struct {
	Status     string               `json:"status"`
	Candidates []types.SearchResult `json:"candidates"`
}

type mixTrackResult struct {
	Index      int    `json:"index"`
	Artist     string `json:"artist"`
	Title      string `json:"title"`
	Timestamp  string `json:"timestamp,omitempty"`
	Status     string `json:"status"`
	OutputFile string `json:"output_file,omitempty"`
	Error      string `json:"error,omitempty"`
}
type mixResult struct {
	Status    string              `json:"status"`
	Title     string              `json:"title"`
	SourceURL string              `json:"source_url,omitempty"`
	Tracks    []mixTrackResult    `json:"tracks"`
	Skipped   []types.SkippedItem `json:"skipped"`
}

func finishMix(ctx context.Context, opts downloader.DownloadOptions, sourceURL string) error {
	if !jsonOutput {
		return downloader.DownloadSet(ctx, opts)
	}
	manifest := &downloader.MixManifest{}
	opts.Result = manifest
	opts.Output = os.Stderr
	err := downloader.DownloadSet(ctx, opts)
	result := mixResult{Status: "success", Title: opts.MixTitle, SourceURL: sourceURL, Tracks: []mixTrackResult{}, Skipped: opts.SkippedItems}
	if opts.DryRun {
		result.Status = "preview"
	}
	if err != nil {
		result.Status = "error"
	}
	for _, entry := range manifest.Tracks {
		file := ""
		if entry.ActualFile != "" {
			file = filepath.Join(manifest.TargetDir, entry.ActualFile)
		}
		if entry.Status == "failed" {
			result.Status = "partial"
		}
		result.Tracks = append(result.Tracks, mixTrackResult{Index: entry.Index, Artist: entry.Artist, Title: entry.Title, Timestamp: entry.Timestamp, Status: entry.Status, OutputFile: file, Error: entry.ErrorMessage})
	}
	return writeJSONResult(result, err)
}
