A command-line tool with AI agent support for bedroom and amateur DJs to find, extract, and download full tracklists from DJ sets, mixes, and YouTube videos.

# What It Does

- Finds full DJ mix tracklists by searching MixesDB and crawlable tracklist web mirrors (OpeningTrack, Tracklist.club, Thomas Laupstad).
- Extracts tracklists from YouTube video comments using multi-provider LLMs (Ollama, LiteLLM, Google Gemini, OpenAI, Anthropic Claude, OpenRouter, DeepSeek, Groq, custom endpoints) with automatic deterministic fallback.
- Resolves 1001Tracklists URLs automatically by extracting set slugs and discovering unblocked web mirrors.
- Generates order-preserving M3U playlists inside the mix output folder keeping the exact chronological track order of the set.
- Tracks download state with resumable JSON manifests (`mix_manifest.json`) to skip already downloaded tracks when resuming.
- Integrates with `fetch-track` CLI for single-track audio stream downloading, loudness normalization, spectral inspection, and 1400x1400 cover art tagging.

# How It Works

- Searches MixesDB, web mirrors, or YouTube comments for your requested DJ set title or video URL.
- Extracts and cleans structured tracklists using AI or regex fallback rules.
- Previews the download plan and directory structure before downloading when using dry-run mode.
- Downloads individual tracks sequentially with anti-ban pacing delays to prevent rate limits.
- Generates an M3U playlist file referencing downloaded tracks in chronological set order.

# How it Really Works

- Interrogates direct tracklist URLs directly or extracts slugs from 1001tracklists URLs to locate crawlable mirror pages.
- Scrapes web mirrors via MediaWiki APIs or HTML-to-Markdown pipelines and extracts structured artist/title pairs.
- Fetches top YouTube comments using `yt-dlp --no-playlist`, filters candidate comments with separator density heuristics, and parses tracklists via structured LLM schemas.
- Paces single-track downloads sequentially with randomized delays (1.0–2.5s) to prevent IP rate limits on streaming platforms.
- Normalizes timestamp formats across the entire mix (automatically padding to `[hh:mm:ss]` if any track exceeds one hour).
- Writes atomic state entries to `mix_manifest.json` on disk to ensure interrupted mix downloads resume seamlessly without re-downloading.
- Generates `playlist.m3u` using sanitized local file names matching the exact set sequence.
- Streams real-time NDJSON progress events over UNIX domain sockets, TCP, or file descriptors when executed by AI agents or supervisor orchestrators.

# Prerequisites

- [`fetch-track`](https://github.com/alexgorbatchev/fetch-track-cli) (version 1.4.0 or newer) - Single-track acquisition engine.
- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp#installation) (version 2024.08.01 or newer) - YouTube comment and metadata extractor.
- [`ffmpeg`](https://ffmpeg.org/download.html) (version 4.4 or newer) - Audio stream processing and transcoding.

# Installation

Download the prebuilt archive for your platform from [GitHub Releases](https://github.com/alexgorbatchev/fetch-mix-cli/releases/latest), extract the `fetch-mix` binary, and place it in your `$PATH`:

```bash
# Example for macOS (Apple Silicon):
curl -sSL "https://github.com/alexgorbatchev/fetch-mix-cli/releases/latest/download/fetch-mix_darwin_arm64.tar.gz" | tar -xz
chmod +x fetch-mix
mv fetch-mix ~/.local/bin/
```

# Quick Start

```bash
# Download a full DJ mix by title
fetch-mix "Bicep Essential Mix 2014"

# Preview download plan without downloading files
fetch-mix --dry-run "https://www.youtube.com/watch?v=NeH3RpyocNc"

# Extract tracklist from YouTube comments via LLM
fetch-mix youtube "https://www.youtube.com/watch?v=NeH3RpyocNc"

# Inspect supported AI providers and models
fetch-mix ai
fetch-mix ai inspect openai

# Manage and verify external dependencies
fetch-mix dependencies
fetch-mix deps install
fetch-mix deps update

# Upgrade CLI binary in-place
fetch-mix upgrade
```

# Options & Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
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

# Progress & IPC Telemetry

When executing `fetch-mix` from an AI agent or parent supervisor, stream NDJSON events over UNIX sockets, TCP, or file descriptors:

```bash
fetch-mix --progress-target "unix:///tmp/fm.sock" "Bicep Essential Mix 2014"
```

See [PROGRESS.md](PROGRESS.md) for full protocol specifications and event schemas.

# License

MIT License (c) 2026 Alex Gorbatchev
