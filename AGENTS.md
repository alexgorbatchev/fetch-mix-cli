# AGENTS.md — fetch-mix-cli

Operational guidelines, architecture, and developer interface specs for AI agents interacting with or maintaining `fetch-mix`.

---

## 1. Overview & System Purpose

`fetch-mix` is a Go CLI application that searches, extracts, and downloads entire tracklists for DJ sets and mixes. It serves as a set-level orchestrator complementary to [`fetch-track`](https://github.com/alexgorbatchev/fetch-track-cli) (which handles single-track audio acquisition, spectral bandwidth verification, and artwork tagging).

### Core Responsibilities
- **Tracklist Search & Discovery**: Native HTTP queries to MixesDB (MediaWiki Search API) and unblocked web mirrors (OpeningTrack, Tracklist.club) via WordPress REST APIs. Accepts 1001tracklists URLs by extracting set slugs and discovering mirrors.
- **Multi-Provider LLM Integration**: Uses `github.com/zendev-sh/goai` to extract tracklists from comments and web sets across 25+ LLM providers (Ollama 1st priority, LiteLLM 2nd, Google Gemini 3rd, OpenAI 4th, Anthropic Claude, OpenRouter, DeepSeek, Groq, and custom OpenAI-compatible endpoints) with deterministic regex fallback.
- **Intermediary Manifest & Resumable Downloads**: Saves a JSON manifest (`{mix-title}/mix_manifest.json`) tracking download status per track and mapping track indices to actual saved file names on disk. Automatically resumes interrupted downloads.
- **Default Output Layout**: Downloads tracks into `cwd/{mix-title-from-youtube}/01 - Artist - Title.m4a` by default unless `--out-dir` / `-o` is provided.
- **Dry-Run Preview (`--dry-run`)**: Parses tracklists and previews planned track output paths, filenames, and `.m3u` playlist structure without performing network downloads or writing files.
- **Single-Track Delegation**: Delegates actual track downloads, audio stream checks, and cover art tagging to `fetch-track` CLI.

---

## 2. System Architecture & Package Boundaries

```
fetch-mix-cli/
├── cmd/
│   └── fetch-mix/          # Cobra CLI entry point, flag parsing, command routing
└── internal/
    ├── cache/              # File-based JSON caching in .tmp/ or XDG cache
    ├── deps/               # Verification & auto-install of external dependencies (fetch-track, yt-dlp, ffmpeg)
    ├── downloader/         # Sequential download execution & .m3u playlist generation
    ├── llm/                # Multi-provider LLM integration via goai (Ollama, LiteLLM, Gemini, OpenAI, etc.)
    ├── parser/             # LLM-driven tracklist extraction with deterministic regex fallback
    ├── progress/           # Socket progress listener & NDJSON streaming telemetry
    ├── scraper/            # Native HTTP fetching & HTML-to-Markdown / Wikitext extraction
    ├── search/             # Native MixesDB MediaWiki & WordPress REST API search
    ├── types/              # Domain models (Track, SearchResult, ScrapedSet, SkippedItem)
    └── youtube/            # YouTube comment fetching, candidate pre-filtering, LLM extraction
```

### Package Contracts
- **`parser.ParseTracklist(markdown string) ([]types.Track, []types.SkippedItem)`**: Deterministic pure function fallback.
- **`parser.ParseTracklistWithAI(ctx, content, title, provider, model) ([]types.Track, []types.SkippedItem, error)`**: Uses configured LLM for semantic tracklist extraction, falling back to deterministic parser.
- **`search.SearchSet(ctx, query) ([]types.SearchResult, error)`**: Searches MixesDB MediaWiki API first; falls back to crawlable web mirrors.
- **`scraper.ScrapeSet(ctx, url) (string, error)`**: Fetches target URL natively, retrieving raw wikitext for MixesDB or converting HTML to Markdown for general web pages.
- **`youtube.ProcessYouTubeComments(ctx, url, noCache, provider, model) ([]types.Track, []types.SkippedItem, string, error)`**: Fetches comments, uses local cache in `.tmp/`, pre-filters candidates, calls LLM, returns tracks, skipped items, and video title.
- **`downloader.DownloadSet(ctx, opts) error`**: Handles sequential downloading via `fetch-track` CLI, handles `--dry-run` preview, and generates `.m3u` playlist.

---

## 3. Environment & Dependency Requirements

The following external binaries must be available in `$PATH`:

1. **`fetch-track`** (min version `1.4.0`): Single-track downloader pipeline.
2. **`yt-dlp`** (min version `2024.08.01`): Comment fetcher and YouTube video query engine.
3. **`ffmpeg`** (min version `4.4`): Audio stream processor.
4. **`AGENT=1`**: Environment variable enabling agent mode (non-interactive auto-selection, structured machine-readable dependency output).

Verify dependency status at any time:
```bash
fetch-mix dependencies
```

---

## 4. CLI Commands & Flags

### Root Command
```bash
fetch-mix <set_title_or_url>
```
If argument is a YouTube URL or video ID, automatically routes to YouTube comments pipeline.

### YouTube Subcommand
```bash
fetch-mix youtube <youtube_url_or_id>
# Alias:
fetch-mix yt <youtube_url_or_id>
```

### AI Subcommand
```bash
fetch-mix ai
# Aliases:
fetch-mix providers
fetch-mix models
```
Prints supported LLM providers, default models, required environment variables, and active API key detection status.

### Dependencies Subcommand
```bash
fetch-mix dependencies
# Aliases:
fetch-mix deps

# Auto-install missing dependencies to ~/.local/share/fetch-mix/bin
fetch-mix deps install [dep...]

# Update dependencies to their latest versions
fetch-mix deps update [dep...]
```

### Upgrade Subcommand
```bash
fetch-mix upgrade
# Aliases:
fetch-mix self-update
fetch-mix update-self
```
Upgrades the `fetch-mix` binary itself in-place from GitHub releases without using GitHub API.

### Flags
| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--out-dir` | `-o` | `cwd/{mix-title}` | Custom output directory |
| `--dry-run` | | `false` | Preview tracklist and download plan without downloading |
| `--auto-install` | | `false` | Automatically install missing dependencies without prompting |
| `--no-cache` | | `false` | Disable local caching for search, tracklists, and comments |
| `--llm-provider` | `-p` | `auto` | LLM provider name (`auto`, `ollama`, `litellm`, `gemini`, `openai`, `anthropic`, `openrouter`, `deepseek`, `groq`, `custom`) |
| `--llm-model` | `-m` | | LLM model name override (e.g. `gpt-4o-mini`, `claude-3-5-haiku-latest`, `llama3.2`) |
| `--sources` | `-s` | `youtube,soundcloud` | Comma-separated search sources passed to `fetch-track` |
| `--interactive` | `-i` | `false` | Interactively choose set search result |
| `--progress-target` | | `""` | Target URI for streaming NDJSON progress events (`unix:///path.sock`, `tcp://127.0.0.1:9099`, `fd://3`, `stdout`, `stderr`) |
| `--progress-socket` | | `""` | Shorthand alias for `--progress-target` |
| `--skip-verify` | | `false` | Skip audio quality spectrum check in `fetch-track` |
| `--skip-metadata` | | `false` | Skip cover art and metadata tagging in `fetch-track` |
| `--verbose` | `-v` | `false` | Enable verbose log output |
| `--version` | | | Prints raw version string (`dev\n`) |

---

## 5. Output Directory & Playlist Specification

- **Default Path**: `cwd/{mix-title-from-youtube}/`
- **Track Files**: `01 - Artist - Title.m4a`, `02 - Artist - Title.m4a`, ...
- **Playlist File**: `{mix-title}/playlist.m3u`
- **Manifest File**: `{mix-title}/mix_manifest.json`
- **Playlist Format**:
```m3u
#EXTM3U
#EXTINF:-1,Artist 1 - Title 1
01 - Artist 1 - Title 1.m4a
#EXTINF:-1,Artist 2 - Title 2
02 - Artist 2 - Title 2.m4a
```

---

## 6. Development & Verification Commands

```bash
# Build binary to bin/fetch-mix
just build

# Run unit tests across all packages
just test

# Run static code analysis
just vet

# Format code
just fmt
```
