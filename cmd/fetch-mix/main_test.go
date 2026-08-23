package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/search"
)

func setupMockLLMServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-cli",
			"object": "chat.completion",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"found\": true, \"tracks\": [{\"artist\": \"Artist 1\", \"title\": \"Track 1\", \"timestamp\": \"00:00\"}, {\"artist\": \"Artist 2\", \"title\": \"Track 2\", \"timestamp\": \"05:00\"}], \"skippedItems\": []}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test_key")
	return server
}

func TestCLI_Subcommands(t *testing.T) {
	ctx := context.Background()

	// 1. Version flag
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--version"})
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("--version failed: %v", err)
	}

	// 2. AI subcommand
	cmd = newRootCmd()
	cmd.SetArgs([]string{"ai"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("ai subcommand failed: %v", err)
	}

	// 3. Dependencies subcommand (with mock cache so all deps pass)
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
	}

	// Non-agent mode
	t.Setenv("AGENT", "0")
	cmd = newRootCmd()
	cmd.SetArgs([]string{"dependencies"})
	_ = cmd.ExecuteContext(ctx)

	// Agent mode
	t.Setenv("AGENT", "1")
	cmd = newRootCmd()
	cmd.SetArgs([]string{"dependencies"})
	_ = cmd.ExecuteContext(ctx)

	// 4. Deps install and update with multiple args
	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "install", "fetch-track", "yt-dlp"})
	_ = cmd.ExecuteContext(ctx)

	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "update", "fetch-track", "yt-dlp"})
	_ = cmd.ExecuteContext(ctx)

	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "install"})
	_ = cmd.ExecuteContext(ctx)

	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "update"})
	_ = cmd.ExecuteContext(ctx)

	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "install", "invalid-dep"})
	_ = cmd.ExecuteContext(ctx)

	cmd = newRootCmd()
	cmd.SetArgs([]string{"deps", "update", "invalid-dep"})
	_ = cmd.ExecuteContext(ctx)
}

func TestCLI_UpgradeSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	tsUpgrade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			http.Redirect(w, r, "/alexgorbatchev/fetch-mix-cli/releases/tag/v2.0.0", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer tsUpgrade.Close()

	ctx := context.Background()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"upgrade"})
	_ = cmd.ExecuteContext(ctx)
}

func TestCLI_ResolveProgressReporter(t *testing.T) {
	ctx := context.Background()

	// Empty
	progressTarget = ""
	progressSocket = ""
	t.Setenv("FETCH_MIX_PROGRESS_TARGET", "")
	rep, err := resolveProgressReporter(ctx)
	if err != nil || rep != nil {
		t.Errorf("expected nil reporter for empty target")
	}

	// Valid stdout target
	progressTarget = "stdout"
	rep, err = resolveProgressReporter(ctx)
	if err != nil {
		t.Fatalf("resolveProgressReporter failed: %v", err)
	}
	if rep != nil {
		_ = rep.Close()
	}
	progressTarget = ""
}

func TestCLI_RunYouTubePipeline(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Pre-populate YouTube tracks cache and deps cache
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("yt_tracks_testvideoid.json", map[string]interface{}{
			"videoId":    "testvideoid",
			"videoTitle": "Mock Video Title",
			"tracks": []map[string]string{
				{"artist": "Artist 1", "title": "Track 1", "timestamp": "00:00"},
			},
		})
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
	}

	ctx := context.Background()

	// 1. Dry run
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "https://www.youtube.com/watch?v=testvideoid"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("dry-run youtube pipeline failed: %v", err)
	}

	// 2. YouTube subcommand dry run
	cmd = newRootCmd()
	cmd.SetArgs([]string{"youtube", "--dry-run", "testvideoid"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("youtube subcommand dry run failed: %v", err)
	}

	// 3. YouTube non-dry-run
	dryRun = false
	outDir = filepath.Join(tempDir, "yt_out")
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = runYouTubePipeline(canceledCtx, "testvideoid")
	dryRun = true
}

func TestCLI_RunMixPipeline_DirectURL(t *testing.T) {
	llmSrv := setupMockLLMServer(t)
	defer llmSrv.Close()

	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Pre-populate dependencies in cache
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
	}

	// Mock web page server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<h1>DJ Set</h1>
			<h2>Tracklist</h2>
			<ul>
				<li>01. Artist 1 - Track 1</li>
				<li>02. Artist 2 - Track 2</li>
			</ul>
		`))
	}))
	defer server.Close()

	ctx := context.Background()

	// 1. Dry run
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", server.URL + "/my-direct-set.html"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("direct URL dry-run failed: %v", err)
	}

	// 2. Dry run with out-dir
	cmd = newRootCmd()
	outDir := filepath.Join(tempDir, "direct_out")
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", "-o", outDir, server.URL + "/my-direct-set.html"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("direct URL dry-run with out-dir failed: %v", err)
	}

	// 3. Dry run with underscore slug and no html extension
	cmd = newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", server.URL + "/my_direct_set_slug"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("direct URL dry-run with slug failed: %v", err)
	}
}

func TestCLI_RunMixPipeline_SearchFlow(t *testing.T) {
	llmSrv := setupMockLLMServer(t)
	defer llmSrv.Close()

	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Mock search and scrape server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/w/api.php"):
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"query": {
					"search": [
						{"pageid": 1, "title": "2024-01-01 - Test Set 1"},
						{"pageid": 2, "title": "2024-01-02 - Test Set 2"}
					]
				}
			}`))
		case strings.HasPrefix(r.URL.Path, "/w/"):
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`== Tracklist ==
# [00:00] Artist 1 - Track 1
# [05:00] Artist 2 - Track 2`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	search.SetMixesDBBaseURLForTest(mockServer.URL)
	defer search.SetMixesDBBaseURLForTest("https://www.mixesdb.com")

	ctx := context.Background()

	// 1. Multiple results in non-interactive mode
	t.Setenv("AGENT", "1")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", "Test Mix Query"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("search pipeline failed: %v", err)
	}

	// 2. Interactive flag enabled with valid choice "2"
	t.Setenv("AGENT", "0")
	interactive = true
	r, w, _ := os.Pipe()
	_, _ = fmt.Fprintln(w, "2")
	_ = w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		interactive = false
	}()

	cmd = newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", "-i", "Test Mix Query"})
	_ = cmd.ExecuteContext(ctx)

	// 3. Interactive flag enabled with invalid choice "invalid_choice"
	r2, w2, _ := os.Pipe()
	_, _ = fmt.Fprintln(w2, "invalid_choice")
	_ = w2.Close()
	os.Stdin = r2

	cmd = newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", "-i", "Test Mix Query"})
	_ = cmd.ExecuteContext(ctx)

	// 4. Progress reporter enabled
	progressTarget = "stdout"
	cmd = newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom", "Test Mix Query"})
	_ = cmd.ExecuteContext(ctx)
	progressTarget = ""
}

func TestCLI_PipelineExtraErrors(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// 1. SearchSet error in runMixPipeline
	serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer serverErr.Close()

	search.SetMixesDBBaseURLForTest(serverErr.URL)
	search.SetMirrorsBaseURLForTest(serverErr.URL, serverErr.URL)
	defer func() {
		search.SetMixesDBBaseURLForTest("https://www.mixesdb.com")
		search.SetMirrorsBaseURLForTest("https://www.openingtrack.com/wp-json/wp/v2/posts", "https://tracklist.club/wp-json/wp/v2/posts")
	}()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "Search Error Set"})
	if err := cmd.ExecuteContext(ctx); err == nil {
		t.Errorf("expected search error")
	}

	// 2. resolveProgressReporter error in runMixPipeline
	progressTarget = "fd://invalid"
	defer func() { progressTarget = "" }()

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("== Tracklist ==\n# Artist - Track"))
	}))
	defer serverOk.Close()

	if err := runMixPipeline(ctx, serverOk.URL+"/set.html"); err == nil {
		t.Errorf("expected progress reporter error in runMixPipeline")
	}

	// 3. resolveProgressReporter error in runYouTubePipeline
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("yt_tracks_repvid.json", map[string]interface{}{
			"videoId":    "repvid",
			"videoTitle": "Video",
			"tracks":     []map[string]string{{"artist": "A", "title": "B"}},
		})
	}
	if err := runYouTubePipeline(ctx, "repvid"); err == nil {
		t.Errorf("expected progress reporter error in runYouTubePipeline")
	}
}

func TestEnsureDependencies_Direct(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// 1. All satisfied
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
	}

	if err := ensureDependencies(ctx); err != nil {
		t.Fatalf("ensureDependencies failed when satisfied: %v", err)
	}

	// 2. Missing dependencies in agent mode
	_ = c.Delete("deps_fetch-track.json")
	_ = c.Delete("deps_yt-dlp.json")
	_ = c.Delete("deps_ffmpeg.json")

	t.Setenv("AGENT", "1")
	if err := ensureDependencies(ctx); err == nil {
		t.Errorf("expected error in agent mode when deps missing")
	}

	// 3. Missing dependencies with autoInstall=true
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	autoInstall = true
	_ = ensureDependencies(canceledCtx)
	autoInstall = false

	// 4. Missing dependencies in interactive mode answering "y"
	t.Setenv("AGENT", "0")
	r, w, _ := os.Pipe()
	_, _ = fmt.Fprintln(w, "y")
	_ = w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_ = ensureDependencies(canceledCtx)

	// 6. Missing dependencies in interactive mode pressing Enter (default Yes)
	r3, w3, _ := os.Pipe()
	_, _ = fmt.Fprintln(w3, "")
	_ = w3.Close()
	os.Stdin = r3
	_ = ensureDependencies(canceledCtx)
}

func TestCLI_Dependencies_Failures(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "0.1.0") // Outdated
		_ = c.Delete("deps_yt-dlp.json")             // Missing / error
	}

	ctx := context.Background()

	// Non-agent mode failure
	t.Setenv("AGENT", "0")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"dependencies"})
	_ = cmd.ExecuteContext(ctx)

	// Agent mode failure
	t.Setenv("AGENT", "1")
	cmd = newRootCmd()
	cmd.SetArgs([]string{"dependencies"})
	_ = cmd.ExecuteContext(ctx)
}

func TestRunMain(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	outDir = tempDir

	// 1. Success with --version
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		outDir = ""
	}()

	os.Args = []string{"fetch-mix", "--version"}
	if code := runMain(ctx); code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// 2. Canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	os.Args = []string{"fetch-mix", "--dry-run", "nonexistent"}
	_ = runMain(canceledCtx)
}

func TestCLI_RunPipelines_FullSuccess(t *testing.T) {
	llmSrv := setupMockLLMServer(t)
	defer llmSrv.Close()

	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Pre-populate dependencies in cache
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
		_ = c.Put("yt_tracks_fullsuccessid.json", map[string]interface{}{
			"videoId":    "fullsuccessid",
			"videoTitle": "Full Success Video",
			"tracks": []map[string]string{
				{"artist": "Artist 1", "title": "Track 1", "timestamp": "01:05:00"},
				{"artist": "Artist 2", "title": "Track 2"},
			},
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("== Tracklist ==\n# [01:05:00] Artist 1 - Track 1\n# Artist 2 - Track 2"))
	}))
	defer server.Close()

	ctx := context.Background()

	// 1. YouTube non-dry-run full success
	dryRun = false
	outDir = filepath.Join(tempDir, "yt_success_out")
	_ = runYouTubePipeline(ctx, "fullsuccessid")

	// 2. Mix non-dry-run full success
	outDir = filepath.Join(tempDir, "mix_success_out")
	_ = runMixPipeline(ctx, server.URL+"/success-set.html")
	dryRun = true
}

func TestCLI_Prompts_Success(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	c, _ := cache.New()
	if c != nil {
		_ = c.Put("deps_fetch-track.json", "2.0.0")
		_ = c.Put("deps_yt-dlp.json", "2025.01.01")
		_ = c.Put("deps_ffmpeg.json", "ffmpeg version 6.0")
		_ = c.Put("yt_tracks_promptsuccess.json", map[string]interface{}{
			"videoId":    "promptsuccess",
			"videoTitle": "Prompt Video Title",
			"tracks": []map[string]string{
				{"artist": "Artist 1", "title": "Track 1", "timestamp": "00:00"},
			},
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("== Tracklist ==\n# Artist - Track"))
	}))
	defer server.Close()

	ctx := context.Background()

	// 1. Root command prompt entering direct URL
	r, w, _ := os.Pipe()
	_, _ = fmt.Fprintln(w, server.URL+"/prompt_set.html")
	_ = w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--dry-run", "-p", "custom"})
	_ = cmd.ExecuteContext(ctx)

	// 2. YouTube command prompt entering cached video ID
	r2, w2, _ := os.Pipe()
	_, _ = fmt.Fprintln(w2, "promptsuccess")
	_ = w2.Close()
	os.Stdin = r2

	cmd = newRootCmd()
	cmd.SetArgs([]string{"youtube", "--dry-run", "-p", "custom"})
	_ = cmd.ExecuteContext(ctx)
}
