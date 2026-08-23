# fetch-mix

`fetch-mix` is a command-line tool with AI agent support for bedroom and amateur DJs to find, extract, and download full tracklists from DJ sets, mixes, and YouTube videos.

> **⚠️ Intended Audience & Legal Disclaimer:**  
> `fetch-mix` is strictly intended for **amateur and bedroom DJs** practicing at home or playing non-commercial sets who are not seeking to become professional DJs. **Working professional DJs and commercial performers must source music from legitimate commercial sources** (such as Beatport, Bandcamp purchases, Juno Download, iTunes, or authorized record pools).

## What It Does

- **Finds Full DJ Mix Tracklists**: Searches [MixesDB](https://www.mixesdb.com) and crawlable tracklist web mirrors (OpeningTrack, Brizm, Thomas Laupstad, Tracklist.club) for complete DJ set tracklists.
- **Extracts Tracklists from YouTube Comments**: Retrieves video comments via `yt-dlp` and uses multi-provider LLMs (Google Gemini, OpenAI, Anthropic Claude, OpenRouter, DeepSeek, Groq, Ollama, custom endpoints via `goai`) to extract and clean full set tracklists.
- **1001Tracklists URL Intelligence**: Resolves `1001tracklists.com` URLs automatically by extracting set slugs and discovering unblocked web mirrors.
- **Generates Order-Preserving M3U Playlists**: Creates a `playlist.m3u` file inside the mix output folder keeping the exact chronological track order of the set.
- **Resumable Downloads & Tracking**: Maintains a `mix_manifest.json` file inside the mix folder mapping track titles to actual downloaded file names and resuming interrupted downloads without re-downloading existing tracks.
- **Delegates Single Tracks to `fetch-track`**: Uses `fetch-track` CLI under the hood for audio stream downloading, loudness normalization, spectral inspection, and 1400x1400 cover art tagging.

## How It Works

1. **Search**: Searches MixesDB, web mirrors, or YouTube comments for requested DJ set or video URL.
2. **Extract**: Parses tracklists deterministically or extracts valid artist/track pairs using Gemini 2.5 Flash API.
3. **Plan**: Organizes tracks sequentially and previews filenames and target directories in `--dry-run` mode.
4. **Download**: Calls `fetch-track` CLI sequentially for each track in the mix.
5. **Playlist & Manifest**: Updates `mix_manifest.json` with actual saved file paths and builds `playlist.m3u`.

## How It Really Works

### 1. DJ Set Tracklist Search & Web Scraping

- **Direct Tracklist URLs**: If given a direct link to a supported tracklist site (MixesDB, Brizm, OpeningTrack), `fetch-mix` skips search and immediately scrapes the target page.
- **1001Tracklists URLs**: `fetch-mix` detects 1001tracklists links, extracts the set title slug from the URL path (for example, turning `polo-and-pan-cercle-2018.html` into `"polo and pan cercle 2018"`), and searches the web for unblocked mirror pages.
- **Hierarchical Web Search**: `fetch-mix` first queries MixesDB via web search. If MixesDB has a matching set page, it selects that page; if MixesDB has no record, it falls back to a general web search across alternative unblocked tracklist mirrors (OpeningTrack, Brizm, Thomas Laupstad, Tracklist.club).
- **Markdown Conversion & Deterministic Parsing**: Target web pages are scraped and converted into Markdown. A deterministic parser processes the document line by line:
  - Locates the "Tracklist" section while ignoring surrounding noise (related mixes, comments, footers).
  - Strips timestamps, play buttons, list numbers/bullets, record label brackets, and Markdown hyperlinks.
  - Normalizes whitespace and dash separators (hyphens, en-dashes, em-dashes).
  - Splits each line into `Artist` and `Title`.
  - Filters out placeholder entries (like `ID - ID` or `Untitled`) and records them in a skipped items summary.

### 2. YouTube Comment Tracklist AI Extraction

- **YouTube Video Route**: Passing a YouTube video URL or 11-character video ID automatically routes execution to the YouTube comment extraction engine.
- **Fetching & Caching Comments**: `fetch-mix` retrieves top comments (up to 200) from YouTube using `yt-dlp` with single-video flags (`--no-playlist`) to ensure quick response times without downloading multi-video playlists. Retrieved comments and extracted tracklists are cached on disk so subsequent runs on the same video ID load instantly.
- **Local Candidate Pre-Filtering**: Before sending data to AI models, a local scanner screens all retrieved comments to filter out 99% of chatter and spam. It selects comments with at least 3 lines, high density of dash/colon separators, and multiple digits. Candidate comments are sorted by vote count.
- **Gemini 2.5 Flash Structured AI Extraction**: Candidate comments are submitted to **Gemini 2.5 Flash** using a strict JSON output schema. The AI identifies the single comment containing the complete DJ set tracklist and returns structured track items (`artist`, `title`, `timestamp`). It also identifies non-track items (speech timestamps like "Thanks speech", chatter, or incomplete entries missing artist names) so they can be logged separately as skipped items.

### 3. Track Formatting & Download Execution

- **Timestamp Normalization**: If any track in the mix exceeds 1 hour in duration, all timestamps across the mix are padded to 8-character `[hh:mm:ss]` format (e.g. `[00:55:39]` through `[01:06:30]`). If under 1 hour, timestamps remain in 5-character `[mm:ss]` format.
- **Resumable Manifest & Output Directory**: Tracks are saved into `cwd/{mix-title-from-youtube}`. A tracking manifest file (`mix_manifest.json`) is maintained inside the mix folder to map each track title to its actual saved file name on disk and track completion status (`pending`, `completed`, `failed`). If a download is interrupted, re-running the command reads the manifest, skips already completed tracks, and resumes from where it stopped.
- **M3U Playlist Generation**: After single-track downloads complete via `fetch-track`, a `playlist.m3u` file is generated inside the mix folder containing the exact saved filenames in the chronological order of the mix.

## Prerequisites

`fetch-mix` requires the following external binary dependencies installed on your system `$PATH`:

- [`fetch-track`](https://github.com/alexgorbatchev/fetch-track-cli) (version 0.1.0 or newer) — for single-track acquisition
- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp#installation) (version 2024.08.01 or newer) — for YouTube comment extraction
- [`ffmpeg`](https://ffmpeg.org/download.html) (version 4.4 or newer) — for audio processing
- [`firecrawl`](https://github.com/alexgorbatchev/firecrawl-cli) (version 1.0.0 or newer) — for web search and tracklist page scraping
- **Gemini API Key** (`GEMINI_API_KEY`) — required for YouTube comments extraction (`fetch-mix youtube`)

## Installation

Go to the [Latest Release Page](https://github.com/alexgorbatchev/fetch-mix-cli/releases/latest) and download the pre-compiled archive for your operating system.

### Build from Source

```bash
git clone https://github.com/alexgorbatchev/fetch-mix-cli.git
cd fetch-mix-cli
just build
```
The compiled binary will be placed in `bin/fetch-mix`.

## Quick Start

### 1. Download a DJ Set Tracklist

```bash
fetch-mix "Bicep Essential Mix 2014"
```

### 2. Preview Downloads with `--dry-run`

```bash
fetch-mix --dry-run "https://www.youtube.com/watch?v=NeH3RpyocNc"
```

Sample Output:
```
Processing YouTube video tracklist comments: https://www.youtube.com/watch?v=NeH3RpyocNc...
Found cached tracklist on disk for YouTube video ID "NeH3RpyocNc"

==================================================
DRY RUN PREVIEW - No files will be downloaded
==================================================
Mix / Set Title   : BORIS REDWALL / СТАНЦИЯ МЕТРО ГОРЬКОВСКАЯ / LA GRANDE FINALE 2025
Target Directory  : BORIS REDWALL - СТАНЦИЯ МЕТРО ГОРЬКОВСКАЯ - LA GRANDE FINALE 2025/
Playlist File     : BORIS REDWALL - СТАНЦИЯ МЕТРО ГОРЬКОВСКАЯ - LA GRANDE FINALE 2025/playlist.m3u
Manifest File     : BORIS REDWALL - СТАНЦИЯ МЕТРО ГОРЬКОВСКАЯ - LA GRANDE FINALE 2025/mix_manifest.json
Total Tracks      : 32
Sources           : youtube,soundcloud

Planned Track Downloads:
  [00:00:30] 01 - BORIS REDWALL - Motor
  [00:01:42] 02 - HELLOVERCAVI, SODA LUV - СВАГА
  [00:03:50] 03 - Boris Redwall - Rude Boy
  [00:04:40] 04 - Whitney Houston - I Wanna Dance With Somebody
  ...
  [01:06:30] 32 - AFELIA - Прости (Sorry)

==================================================
Playlist file preview: BORIS REDWALL - СТАНЦИЯ МЕТРО ГОРЬКОВСКАЯ - LA GRANDE FINALE 2025/playlist.m3u (32 tracks in mix order)
[DRY RUN COMPLETE] Plan verified. No downloads performed.
==================================================
```

### 3. Extract Tracks from YouTube Comments

```bash
# Auto-detects active LLM provider (GEMINI_API_KEY / OPENAI_API_KEY / etc.)
fetch-mix youtube "https://www.youtube.com/watch?v=NeH3RpyocNc"

# Specify explicit LLM provider and model
fetch-mix youtube -p openai -m gpt-4o-mini "https://www.youtube.com/watch?v=NeH3RpyocNc"
fetch-mix youtube -p anthropic -m claude-3-5-haiku-latest "https://www.youtube.com/watch?v=NeH3RpyocNc"
fetch-mix youtube -p ollama -m llama3.2 "https://www.youtube.com/watch?v=NeH3RpyocNc"
```

### 4. Inspect Supported AI Providers and Models

```bash
fetch-mix ai
```

Sample Output:
```
==================================================
Supported LLM Providers & Models:
==================================================
PROVIDER       DEFAULT MODEL              ENV VAR              STATUS
gemini         gemini-2.5-flash           GEMINI_API_KEY       Active
openai         gpt-4o-mini                OPENAI_API_KEY       Active
anthropic      claude-3-5-haiku-latest    ANTHROPIC_API_KEY    Not Set
openrouter     google/gemini-2.5-flash    OPENROUTER_API_KEY   Active
deepseek       deepseek-chat              DEEPSEEK_API_KEY     Not Set
groq           llama-3.3-70b-versatile    GROQ_API_KEY         Not Set
ollama         llama3.2                   OLLAMA_HOST          Not Set (default: http://localhost:11434)
custom         gpt-4o-mini                OPENAI_BASE_URL      Not Set
==================================================
```

### 4. Custom Output Directory

```bash
fetch-mix -o "my_sets/bicep" "Bicep Essential Mix 2014"
```

### 5. Verify Installed Dependencies

Verify that `fetch-track`, `yt-dlp`, `ffmpeg`, and `firecrawl` are installed and meet minimum version requirements:

```bash
fetch-mix dependencies
```

Sample Output:
```
fetch-track: 1.0.0 (min 0.1.0) [OK]
yt-dlp: 2026.07.04 (min 2024.08.01) [OK]
ffmpeg: 8.1.2 (min 4.4) [OK]
firecrawl: 1.0.0 (min 1.0.0) [OK]

All required dependencies are installed and operational.
```

## Options & Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--out-dir` | `-o` | `cwd/{mix-title}` | Folder where mix tracks are saved |
| `--dry-run` | | `false` | Preview tracklist and download plan without downloading |
| `--no-cache` | | `false` | Disable local caching for search queries, tracklists, and comments |
| `--llm-provider` | `-p` | `auto` | LLM provider name (`auto`, `gemini`, `openai`, `anthropic`, `openrouter`, `deepseek`, `groq`, `ollama`, `custom`) |
| `--llm-model` | `-m` | | LLM model name override (e.g. `gpt-4o-mini`, `claude-3-5-haiku-latest`, `llama3.2`) |
| `--sources` | `-s` | `youtube,soundcloud` | Comma-separated search sources passed to `fetch-track` |
| `--interactive` | `-i` | `false` | Interactively choose set search result |
| `--skip-verify` | | `false` | Skip audio quality spectrum check in `fetch-track` |
| `--skip-metadata` | | `false` | Skip cover art and metadata tagging in `fetch-track` |
| `--verbose` | `-v` | `false` | Show extra detailed progress logs |

## License

This project is licensed under the [MIT License](LICENSE).
