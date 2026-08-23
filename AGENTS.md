# AGENTS.md — fetch-mix-cli

Operational guidelines, architecture, and developer interface specs for AI agents interacting with or maintaining `fetch-mix`.

---

## 1. Overview & System Purpose

`fetch-mix` is a Go CLI application that searches, extracts, and downloads entire tracklists for DJ sets and mixes. It serves as a set-level orchestrator complementary to [`fetch-track`](https://github.com/alexgorbatchev/fetch-track-cli) (which handles single-track audio acquisition, spectral bandwidth verification, and artwork tagging).

### Core Responsibilities
- **Tracklist Search & Discovery**: Queries MixesDB and unblocked web mirrors (Brizm, OpeningTrack, Thomas Laupstad, Tracklist.club) via `firecrawl`. Accepts 1001tracklists URLs by extracting set slugs and discovering mirrors.
- **YouTube Comment Intelligence**: Fetches YouTube comments with `yt-dlp` and uses **Gemini 2.5 Flash** with native structured outputs to extract full tracklists from comments.
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
    ├── cache/              # File-based JSON caching in .tmp/ for YouTube comment dumps
    ├── deps/               # Verification of external dependencies (fetch-track, yt-dlp, ffmpeg, firecrawl)
    ├── downloader/         # Sequential download execution & .m3u playlist generation
    ├── parser/             # Deterministic markdown tracklist parser (MixesDB, Brizm, OpeningTrack)
    ├── scraper/            # Firecrawl page scraping wrapper
    ├── search/             # MixesDB & general web tracklist search wrapper
    ├── types/              # Domain models (Track, SearchResult, ScrapedSet)
    └── youtube/            # YouTube comment fetching, candidate pre-filtering, Gemini 2.5 Flash API client
```

### Package Contracts
- **`parser.ParseTracklist(markdown string) []types.Track`**: Pure function. Strips markdown links, timestamps, list numbers, brackets, and filters out placeholders (`ID - ID`, `Untitled`).
- **`search.SearchSet(ctx, query) ([]types.SearchResult, error)`**: Searches MixesDB first; falls back to general crawlable web mirrors.
- **`scraper.ScrapeSet(ctx, url) (string, error)`**: Scrapes target URL using `firecrawl scrape --json`.
- **`youtube.ProcessYouTubeComments(ctx, url) ([]types.Track, string, error)`**: Fetches comments, uses local cache in `.tmp/`, pre-filters candidates, calls Gemini 2.5 Flash API, returns tracks and video title.
- **`downloader.DownloadSet(ctx, opts) error`**: Handles sequential downloading via `fetch-track` CLI, handles `--dry-run` preview, and generates `.m3u` playlist.

---

## 3. Environment & Dependency Requirements

The following external binaries must be available in `$PATH`:

1. **`fetch-track`** (min version `0.1.0`): Single-track downloader pipeline.
2. **`yt-dlp`** (min version `2024.08.01`): Comment fetcher and YouTube video query engine.
3. **`ffmpeg`** (min version `4.4`): Audio stream processor.
4. **`firecrawl`** (min version `1.0.0`): Web scraper and search engine.
5. **`GEMINI_API_KEY`**: Environment variable required for YouTube comment extraction (`fetch-mix youtube`).
6. **`AGENT=1`**: Environment variable enabling agent mode (non-interactive auto-selection, structured machine-readable dependency output).

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

### Dependencies Subcommand
```bash
fetch-mix dependencies
# Alias:
fetch-mix deps
```

### Flags
| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--out-dir` | `-o` | `cwd/{mix-title}` | Custom output directory |
| `--dry-run` | | `false` | Preview tracklist and download plan without downloading |
| `--sources` | `-s` | `youtube,soundcloud` | Comma-separated search sources passed to `fetch-track` |
| `--interactive` | `-i` | `false` | Interactively choose set search result |
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
