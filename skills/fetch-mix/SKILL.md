---
name: fetch-mix
description: Discover DJ mix tracklists and download their tracks and playlists with fetch-mix.
author: alexgorbatchev
metadata:
  created_on: 2026-10-18 09:57
  last_modified: 2026-10-30 11:35
  status: current
---

## Workflow

Run with `AGENT=1` for non-interactive selection and compact output. Preview a set before acquiring it, then reuse its output directory to resume.

```sh
AGENT=1 fetch-mix ai list
AGENT=1 fetch-mix dependencies verify
AGENT=1 fetch-mix --dry-run "Bicep Essential Mix 2014"
AGENT=1 fetch-mix -o tracks/bicep "Bicep Essential Mix 2014"
AGENT=1 fetch-mix youtube --dry-run -p ollama "https://www.youtube.com/watch?v=NeH3RpyocNc"
```

## Commands and arguments

| Invocation | Behavior |
| --- | --- |
| `fetch-mix <url|query>` | Join arguments into a set query or URL. Search MixesDB/mirrors; direct crawlable URLs bypass search. YouTube URLs/IDs route to the YouTube pipeline. No input displays help in text mode and returns an error in JSON mode. |
| `fetch-mix youtube <url|id>` | Alias `yt`; extract a tracklist from video comments. No input displays help in text mode and returns an error in JSON mode. |
| `fetch-mix ai` | Aliases `providers`, `models`; same provider listing as ai list. |
| `fetch-mix ai list` | Alias `ls`; list detection status and default model in provider priority order. |
| `fetch-mix ai inspect <provider>` | Aliases `show`, `get`; inspect one provider's configuration. Exactly one provider is required. |
| `fetch-mix dependencies` | Alias `deps`; verify fetch-track >=1.4.0, yt-dlp >=2024.08.01, ffmpeg >=4.4. |
| `fetch-mix dependencies verify` | Verify installed dependencies and minimum versions. |
| `fetch-mix dependencies install [dep...]` | Install missing named dependencies; omitted names select all. |
| `fetch-mix dependencies update [dep...]` | Update named dependencies; omitted names select all. |
| `fetch-mix self` | Display executable management help. |
| `fetch-mix self upgrade` | Download a GitHub release and replace this executable. |
| `fetch-mix skill` | Print the offline Skills CLI install command and repository URL, without arguments or command-specific flags. |
| `fetch-mix help [command...]` | Display help for a command path. |
| `fetch-mix completion` | Display shell completion help. |
| `fetch-mix completion bash` | Write Bash completion script. |
| `fetch-mix completion zsh` | Write Zsh completion script. |
| `fetch-mix completion fish` | Write Fish completion script. |
| `fetch-mix completion powershell` | Write PowerShell completion script. |

## Flags

Dependency verify aliases are `check` and `status`; install aliases are `add` and `get`; update also accepts `upgrade`.

Root and youtube accept these local flags.

| Flag | Type | Default | Effect |
| --- | --- | --- | --- |
| `--json` | bool | false | Return one structured JSON result on stdout; diagnostics go to stderr. Available globally. |
| `--list-candidates` | bool | false | List source candidates without extraction or downloading. |
| `--candidate-url` | string | "" | Extract the exact selected HTTP(S) source URL without searching again. |
| `--out-dir`, `-o` | string | "" | Empty selects cwd/{sanitized mix title}; explicit path selects destination. |
| `--dry-run` | bool | false | Preview tracks, output paths, and playlist; still fetches tracklist sources, calls AI, and can populate caches. |
| `--no-cache` | bool | false | Bypass cached searches, tracklists, and comments. |
| `--sources`, `-s` | string | youtube,soundcloud | Comma-separated sources forwarded to fetch-track. |
| `--skip-verify` | bool | false | Skip delegated audio quality verification. |
| `--skip-metadata` | bool | false | Skip delegated tags/artwork. |
| `--llm-provider`, `-p` | string | auto | auto, ollama, litellm, gemini, openai, anthropic, openrouter, deepseek, groq, custom. |
| `--llm-model`, `-m` | string | "" | Empty selects provider default below. |
| `--progress-target` | string | "" | unix:///path.sock, tcp://host:port, fd://3, stdout, stderr. |
| `--progress-socket` | string | "" | Socket shorthand, lower precedence than explicit target. |
| `--auto-install` | bool | false | Install missing dependencies without prompting. |

Root-only `--interactive` / `-i` (bool, default false) prompts to select a search result in human mode; agent mode always selects the first. Global `--verbose` / `-v` (bool, default false) enables diagnostic logs. All commands accept `--help` / `-h` (bool, default false). Root `--version` (bool, default false) prints raw version plus newline. Shell completion offers `--no-descriptions` (bool, default false) where supported.

## Providers and environment

Auto-selection uses the first detected provider in this order:

| Provider | Default model | Detection variable |
| --- | --- | --- |
| ollama | llama3.2 | OLLAMA_HOST or OLLAMA_URL |
| litellm | gemini-2.5-flash | LITELLM_BASE_URL, LITELLM_URL, or LITELLM_API_KEY |
| gemini | gemini-2.5-flash | GEMINI_API_KEY |
| openai | gpt-4o-mini | OPENAI_API_KEY |
| anthropic | claude-3-5-haiku-latest | ANTHROPIC_API_KEY |
| openrouter | google/gemini-2.5-flash | OPENROUTER_API_KEY |
| deepseek | deepseek-chat | DEEPSEEK_API_KEY |
| groq | llama-3.3-70b-versatile | GROQ_API_KEY |
| custom | gpt-4o-mini | OPENAI_BASE_URL |

OLLAMA_MODEL and LITELLM_MODEL override their default models. Explicit Ollama defaults to http://localhost:11434. LiteLLM resolves endpoint from LITELLM_BASE_URL, LITELLM_URL, LITELLM_API_BASE, then http://localhost:4000; credentials from LITELLM_API_KEY or LITELLM_KEY. Custom uses OPENAI_BASE_URL and OPENAI_API_KEY. Detection is based on configured variables, not a connectivity probe.

AGENT values 1, true, yes enable agent mode. FETCH_MIX_PROGRESS_TARGET supplies progress destination after explicit flags. FETCH_MIX_DEV=1 or true, or an existing local .tmp directory, selects .tmp caching; otherwise caching uses the platform user cache directory under fetch-mix (XDG_CACHE_HOME on platforms honoring it). PATH locates external tools. Managed dependencies use XDG_DATA_HOME/fetch-mix/bin or ~/.local/share/fetch-mix/bin.

## Output, errors, and side effects

- Human output uses labeled provider entries and full-width terminal separators. Agent provider lists are TSV; pipeline results and status are compact lines. Help writes stdout; errors write stderr. Every agent help screen starts with a skill-reading alert. Displayed home paths use ~.
- Search selects the first result unless human interactive selection is requested. Downloads run sequentially through fetch-track. Tracklist pages/comments may be sent to the selected AI provider; provider charges depend on that account.
- Output contains numbered audio filenames, mix_manifest.json, and playlist.m3u. Resume requires the same mix name and ordered artist, title, search query, and timestamp values; completed nonempty files are skipped. A mismatch stops acquisition without replacing the manifest; select another --out-dir. Files and progress payloads contain actual paths; display folding changes presentation only.
- Dry-run previews do not acquire audio or write the output manifest/playlist, but network and cache activity still occur. Dependency installation and self upgrade change installed executables.
- Progress streams NDJSON events over the requested transport. A progress setup error stops acquisition. The fetch-mix event protocol is independent of tag-track's job envelope.
- Missing dependencies, invalid providers, empty search results, extraction failures, download failures, and playlist-write failures return errors. Failed downloads do not stop attempts for the remaining tracks; the command exits 1 after saving results. Agent output reports status: error, and the final complete progress event includes error text and downloaded/failed counts. Retry the same set and destination after resolving the cause. Unreadable or malformed existing manifests are rejected without overwriting them; repair the manifest or choose a new output directory.

