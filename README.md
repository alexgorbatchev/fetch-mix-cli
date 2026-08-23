# fetch-mix

`fetch-mix` is a command-line tool with AI agent support for bedroom and amateur DJs to find, extract, and download full tracklists from DJ sets, mixes, and YouTube videos.

> **⚠️ Intended Audience & Legal Disclaimer:**  
> `fetch-mix` is strictly intended for **amateur and bedroom DJs** practicing at home or playing non-commercial sets who are not seeking to become professional DJs. **Working professional DJs and commercial performers must source music from legitimate commercial sources** (such as Beatport, Bandcamp purchases, Juno Download, iTunes, or authorized record pools).

## What It Does

- **Finds Full DJ Mix Tracklists**: Searches [MixesDB](https://www.mixesdb.com) and crawlable tracklist web mirrors (OpeningTrack, Brizm, Thomas Laupstad, Tracklist.club) for complete DJ set tracklists.
- **Extracts Tracklists from YouTube Comments**: Retrieves video comments via `yt-dlp` and uses **Gemini 2.5 Flash** with native structured outputs to extract and clean full set tracklists.
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
fetch-mix youtube "https://www.youtube.com/watch?v=NeH3RpyocNc"
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
| `--sources` | `-s` | `youtube,soundcloud` | Comma-separated search sources passed to `fetch-track` |
| `--interactive` | `-i` | `false` | Interactively choose set search result |
| `--skip-verify` | | `false` | Skip audio quality spectrum check in `fetch-track` |
| `--skip-metadata` | | `false` | Skip cover art and metadata tagging in `fetch-track` |
| `--verbose` | `-v` | `false` | Show extra detailed progress logs |

## License

This project is licensed under the [MIT License](LICENSE).
