`fetch-mix` helps bedroom and amateur DJs find, extract, and download full tracklists from DJ sets, mixes, and YouTube videos.

# What It Does

- Finds full DJ mix tracklists by searching MixesDB and crawlable tracklist web mirrors (OpeningTrack, Tracklist.club, Thomas Laupstad).
- Extracts tracklists from YouTube video comments using multi-provider LLMs (Ollama, LiteLLM, Google Gemini, OpenAI, Anthropic Claude, OpenRouter, DeepSeek, Groq, custom endpoints) with automatic deterministic fallback.
- Resolves 1001Tracklists URLs automatically by extracting set slugs and discovering unblocked web mirrors.
- Generates order-preserving M3U playlists inside the mix output folder keeping the exact chronological track order of the set.
- Tracks download state with resumable JSON manifests (`mix_manifest.json`) to skip already downloaded tracks when resuming.
- Lists source candidates without downloading and accepts an exact selected URL for subsequent extraction.
- Returns structured JSON for mix previews/downloads, candidate discovery, AI configuration, dependency management, and upgrades.
- Integrates with `fetch-track` CLI for single-track audio stream downloading, loudness normalization, spectral inspection, and 1400x1400 cover art tagging.

# How It Works

- Searches MixesDB, web mirrors, or YouTube comments for your requested DJ set title or video URL.
- Extracts and cleans structured tracklists using AI or regex fallback rules.
- Previews the download plan and directory structure before downloading when using dry-run mode.
- Downloads individual tracks sequentially with anti-ban pacing delays to prevent rate limits.
- Generates an M3U playlist file referencing downloaded tracks in chronological set order.

# How it Really Works

- Interrogates direct tracklist URLs directly or extracts slugs from 1001tracklists URLs to locate crawlable mirror pages.
- With `--candidate-url`, fetches the selected URL directly, including 1001Tracklists pages; normal 1001Tracklists input searches mirrors. Direct access can fail if the selected site blocks requests.
- Scrapes web mirrors via MediaWiki APIs or HTML-to-Markdown pipelines and extracts structured artist/title pairs.
- Fetches top YouTube comments using `yt-dlp --no-playlist`, filters candidate comments with separator density heuristics, and parses tracklists via structured LLM schemas.
- Paces single-track downloads sequentially with randomized delays (1.0–2.5s) to prevent IP rate limits on streaming platforms.
- Normalizes timestamp formats across the entire mix (automatically padding to `[hh:mm:ss]` if any track exceeds one hour).
- Saves state in `mix_manifest.json` so repeat runs skip completed files. Any failed download or playlist write makes the command exit with code 1 after attempting the remaining tracks. Retry the same set to retry failed tracks. Unreadable or malformed existing manifests are rejected without overwriting them; repair the manifest or choose a new output directory.
- Generates `playlist.m3u` using sanitized local file names matching the exact set sequence.
- Streams real-time NDJSON progress events over UNIX domain sockets, TCP, or file descriptors when executed by AI agents or supervisor orchestrators.
- With `--json`, writes one result to stdout and diagnostics to stderr. Mix results include the source URL and ordered track statuses, output files, and errors; `--dry-run` returns a `preview` without downloading audio. Failed downloads remain visible in a `partial` result.
- Missing targets, invalid candidate URLs or providers, dependency failures, and extraction or progress initialization errors return a JSON error result and a nonzero exit status. A failed download keeps its existing result without appending another JSON document.

# Installation

Download the prebuilt archive for your platform from [GitHub Releases](https://github.com/alexgorbatchev/fetch-mix-cli/releases/latest), extract the `fetch-mix` binary, and place it in your `$PATH`:

```bash
# Example for macOS (Apple Silicon):
curl -fsSL "https://github.com/alexgorbatchev/fetch-mix-cli/releases/download/v2.0.0/fetch-mix_2.0.0_darwin_arm64.tar.gz" | tar -xz
chmod +x fetch-mix
mkdir -p ~/.local/bin
mv fetch-mix ~/.local/bin/
```

# Setup

- Install [`fetch-track`](https://github.com/alexgorbatchev/fetch-track-cli) version 1.4.0 or newer for single-track acquisition.
- Install [`yt-dlp`](https://github.com/yt-dlp/yt-dlp#installation) version 2024.08.01 or newer for YouTube comments.
- Install [`ffmpeg`](https://ffmpeg.org/download.html) version 4.4 or newer for audio processing.
- Configure an AI provider using the [provider reference](skills/fetch-mix/SKILL.md#providers-and-environment).

# Quick Start

```bash
# Download a full DJ mix by title
fetch-mix "Bicep Essential Mix 2014"

# Preview download plan without downloading files
fetch-mix --dry-run "https://www.youtube.com/watch?v=NeH3RpyocNc"

# Extract tracklist from YouTube comments via LLM
fetch-mix youtube "https://www.youtube.com/watch?v=NeH3RpyocNc"

# List matching source URLs, then extract the selected source without searching again
fetch-mix --json --list-candidates "Bicep Essential Mix 2014"
fetch-mix --json --dry-run --candidate-url "https://www.youtube.com/watch?v=NeH3RpyocNc" "Bicep Essential Mix 2014"

# Inspect supported AI providers and models
fetch-mix ai
fetch-mix ai inspect openai

# Manage and verify external dependencies
fetch-mix dependencies
fetch-mix deps install
fetch-mix deps update

# Upgrade CLI binary in-place
fetch-mix self upgrade
```

Sample Output (dry-run with a two-track test set, excerpt):

```text
Target Directory  : 2024-01-02 - Test Set 2/
Playlist File     : 2024-01-02 - Test Set 2/playlist.m3u
Total Tracks      : 2
Sources           : youtube,soundcloud

Planned Track Downloads:
  [00:00] 01 - Artist 1 - Track 1
  [05:00] 02 - Artist 2 - Track 2
```

# Options & Flags

Global options are `--verbose` / `-v` and `--help` / `-h`. Root `--version` prints only the version. Run `fetch-mix skill` for installation instructions for the [agent usage skill](skills/fetch-mix/SKILL.md), including commands, aliases, environment configuration, and side effects.

`fetch-mix` and `fetch-mix youtube` share the flags below, except `--interactive`, which belongs to the root command:

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--json` | | `false` | Return structured results; available on all subcommands |
| `--list-candidates` | | `false` | Return candidate titles and URLs without extraction or downloads; a YouTube URL or video ID resolves to one canonical video candidate |
| `--candidate-url <url>` | | `""` | Extract this exact selected HTTP(S) source URL without searching again |
| `--out-dir <path>` | `-o` | `cwd/{mix-title}` | Output directory where mix tracks are saved |
| `--dry-run` | | `false` | Preview tracklist and download plan without downloading |
| `--auto-install` | | `false` | Automatically install missing dependencies without prompting |
| `--no-cache` | | `false` | Disable local caching for search queries, tracklists, and comments |
| `--llm-provider <name>` | `-p` | `auto` | LLM provider name (`auto`, `ollama`, `litellm`, `gemini`, `openai`, `anthropic`, `openrouter`, `deepseek`, `groq`, `custom`) |
| `--llm-model <name>` | `-m` | `""` | LLM model name override |
| `--sources <list>` | `-s` | `youtube,soundcloud` | Comma-separated search sources passed to fetch-track |
| `--interactive` | `-i` | `false` | Interactively choose set search result |
| `--progress-target <uri>` | | `""` | Target URI for streaming NDJSON progress events |
| `--progress-socket <path>` | | `""` | Shorthand alias for `--progress-target` |
| `--skip-verify` | | `false` | Skip audio quality spectrum check in fetch-track |
| `--skip-metadata` | | `false` | Skip cover art and metadata tagging in fetch-track |
| `--verbose` | `-v` | `false` | Enable verbose diagnostic logging |
| `--version` | | `false` | Print version information and exit |
| `--help` | `-h` | `false` | Print command line help |

The pipeline flags above also apply to `fetch-mix youtube`, except `--interactive`. Use `--list-candidates` to obtain `{status,candidates:[{title,url}]}`. Pass a returned URL with `--candidate-url` when continuing a selection; `--dry-run` then inspects its tracklist without downloading audio.

# Progress & IPC Telemetry

Use `AGENT=1`, `true`, or `yes` for compact output and automatic first-result selection. Human mode offers selection with `--interactive`. Displayed home paths use `~`; filesystem operations and telemetry retain actual paths. Help and results use stdout, command errors use stderr. Dry-run can fetch tracklists, invoke AI, and populate caches, but does not download audio or write the output manifest or playlist.

When executing `fetch-mix` from an AI agent or parent supervisor, stream NDJSON events over UNIX sockets, TCP, or file descriptors:

```bash
fetch-mix --progress-target "unix:///tmp/fm.sock" "Bicep Essential Mix 2014"
```

See [PROGRESS.md](PROGRESS.md) for full protocol specifications and event schemas.

# License

[MIT License](LICENSE) (c) 2026 Alex Gorbatchev
