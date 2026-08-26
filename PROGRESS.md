# Progress & IPC Telemetry Protocol

`fetch-mix` supports streaming real-time, structured progress and telemetry events out-of-band to parent processes (such as other CLI tools, AI agent harnesses, orchestrators, background daemons, or GUI desktop apps).

By separating telemetry from standard output streams, parent processes avoid parsing unstructured terminal text or ANSI escape sequences and can render native progress bars, inspect tracklist extraction live, follow single-track acquisition status, or maintain watchdog heartbeats.

---

## Configuration & Usage

Progress streaming is configured via CLI flags or an environment variable.

### CLI Flags

| Flag | Format / Examples | Description |
| :--- | :--- | :--- |
| `--progress-target` | `unix:///path/to.sock`<br>`tcp://127.0.0.1:9099`<br>`fd://3`<br>`stdout`, `stderr` | Target URI or address where NDJSON progress events are streamed. |
| `--progress-socket` | `/path/to.sock` or `unix:///path/to.sock` | Shorthand alias for `--progress-target`. |

### Environment Variable

If no flag is provided, `fetch-mix` inspects:

```bash
export FETCH_MIX_PROGRESS_TARGET="unix:///tmp/fetch-mix.sock"
```

### Live Demo Recipe

You can run the included out-of-band socket demo recipe directly:

```bash
just demo-progress "https://www.youtube.com/watch?v=NeH3RpyocNc"
```

---

## Supported Target Protocols

### 1. UNIX Domain Sockets (`unix://`)
Best for local parent CLIs, daemon supervisors, and agent harnesses on Linux and macOS.

```bash
# Explicit scheme
fetch-mix --progress-target "unix:///tmp/fetch-mix-progress.sock" "Bicep Essential Mix 2014"

# Shorthand path (auto-detected as unix socket)
fetch-mix --progress-socket "/tmp/fetch-mix-progress.sock" "Bicep Essential Mix 2014"
```

### 2. TCP Sockets (`tcp://`)
Best for cross-platform IPC (including Windows) or distributed orchestrators.

```bash
fetch-mix --progress-target "tcp://127.0.0.1:9099" "Bicep Essential Mix 2014"
```

### 3. Inherited File Descriptors (`fd://`)
Best for POSIX shell subshells and parent processes spawning child processes with extra pipes.

```bash
fetch-mix --progress-target "fd://3" "Bicep Essential Mix 2014" 3> progress.log
```

### 4. Standard Streams (`stdout`, `stderr`)
Best for piping NDJSON events directly into tools like `jq` or logging pipelines.

```bash
fetch-mix --progress-target stderr "Bicep Essential Mix 2014" 2> progress.ndjson
```

---

## Event Stream Specification

Events are streamed as **Newline-Delimited JSON (NDJSON)**. Every line is a single, valid JSON object ending in `\n`.

### Event Object Schema

```typescript
interface ProgressEvent {
  timestamp: string;          // ISO 8601 UTC timestamp (e.g. "2026-08-25T14:35:00Z")
  type: EventType;           // "phase_start" | "candidate_found" | "candidate_selected" | "progress" | "track_start" | "track_complete" | "complete" | "error"
  phase?: string;            // Current pipeline phase ("search" | "scrape" | "youtube_comments" | "parse" | "download" | "playlist" | "complete")
  step?: number;             // Current 1-indexed step/track number
  total_steps?: number;      // Total steps/tracks in execution
  message?: string;          // Human-readable status message
  percent?: number;          // Completion percentage (0.0 to 100.0)
  track?: TrackInfo;         // Populated for set-level track events
  candidate?: CandidateInfo; // Populated for single-track candidate events
  result?: ResultInfo;       // Populated on single-track completion
  summary?: MixSummaryInfo;  // Populated on overall mix completion
  error?: string;            // Error message if type is "error"
}

interface TrackInfo {
  index: number;              // 1-indexed track position in set
  total_tracks: number;       // Total tracks in mix
  artist: string;             // Track artist
  title: string;              // Track title
  timestamp?: string;         // Track timestamp in set (e.g. "01:05:30")
  actual_file?: string;       // Final saved filename on disk (e.g. "01 - Artist - Title.m4a")
  status?: string;            // "pending" | "completed" | "failed" | "skipped"
  error?: string;             // Error message if failed
  duration_seconds?: number;  // Verified track duration
  bandwidth_hz?: number;      // Audio frequency cutoff in Hz
  bandwidth_rating?: string;  // Audio quality classification
  suggested_gain_db?: number; // DJ mixer gain offset in dB
}

interface MixSummaryInfo {
  mix_title: string;          // Name of the DJ mix
  target_dir: string;         // Destination directory on disk
  playlist_path?: string;     // Relative or absolute path to playlist.m3u
  total_tracks: number;       // Total tracks in set
  downloaded: number;         // Successfully downloaded track count
  failed: number;             // Failed track count
  skipped?: number;           // Skipped/ignored items count
}
```

---

## Event Types & Lifecycles

### 1. `phase_start`
Emitted at the beginning of each major pipeline phase (search, scrape, youtube_comments, download, playlist).

```json
{"timestamp":"2026-08-25T14:35:00Z","type":"phase_start","phase":"search","message":"Searching for \"Bicep Essential Mix 2014\"..."}
```

### 2. `track_start`
Emitted when single-track audio acquisition begins for a track in the set.

```json
{"timestamp":"2026-08-25T14:35:10Z","type":"track_start","phase":"download","step":1,"total_steps":12,"message":"Fetching track [01/12]: Bicep - Satisfy","track":{"index":1,"total_tracks":12,"artist":"Bicep","title":"Satisfy","timestamp":"00:00","status":"pending"}}
```

### 3. `candidate_selected`
Bridged from child `fetch-track` when candidate ranking selects the best source stream.

```json
{"timestamp":"2026-08-25T14:35:12Z","type":"candidate_selected","phase":"search","track":{"index":1,"total_tracks":12,"artist":"Bicep","title":"Satisfy"},"candidate":{"title":"Bicep - Satisfy (Official Audio)","source":"youtube","duration_seconds":285}}
```

### 4. `track_complete`
Emitted when a track is successfully downloaded, tagged, verified, and renamed.

```json
{
  "timestamp": "2026-08-25T14:35:18Z",
  "type": "track_complete",
  "phase": "download",
  "step": 1,
  "total_steps": 12,
  "message": "Track completed [01/12]: 01 - Bicep - Satisfy.m4a",
  "track": {
    "index": 1,
    "total_tracks": 12,
    "artist": "Bicep",
    "title": "Satisfy",
    "timestamp": "00:00",
    "actual_file": "01 - Bicep - Satisfy.m4a",
    "status": "completed",
    "duration_seconds": 285,
    "bandwidth_hz": 20000,
    "bandwidth_rating": "High Fidelity (>=18.5 kHz)",
    "suggested_gain_db": -1.2
  }
}
```

### 5. `complete`
Emitted when the entire mix acquisition finishes, including playlist generation.

```json
{
  "timestamp": "2026-08-25T14:40:00Z",
  "type": "complete",
  "phase": "complete",
  "message": "Set download completed: 12 downloaded, 0 failed",
  "summary": {
    "mix_title": "Bicep Essential Mix 2014",
    "target_dir": "Bicep Essential Mix 2014",
    "playlist_path": "Bicep Essential Mix 2014/playlist.m3u",
    "total_tracks": 12,
    "downloaded": 12,
    "failed": 0,
    "skipped": 1
  }
}
```

---

## Integration Examples

### Go Parent Process (UNIX Domain Socket)

```go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

type ProgressEvent struct {
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Track   *struct {
		Index       int    `json:"index"`
		TotalTracks int    `json:"total_tracks"`
		Artist      string `json:"artist"`
		Title       string `json:"title"`
		ActualFile  string `json:"actual_file"`
	} `json:"track,omitempty"`
}

func main() {
	sockPath := filepath.Join(os.TempDir(), "fm_progress.sock")
	defer os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	// Spawn fetch-mix child process
	cmd := exec.Command("fetch-mix", "--progress-socket", sockPath, "Bicep Essential Mix 2014")
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var ev ProgressEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err == nil {
			if ev.Track != nil {
				fmt.Printf("[%s] [%d/%d] %s: %s - %s\n", ev.Phase, ev.Track.Index, ev.Track.TotalTracks, ev.Type, ev.Track.Artist, ev.Track.Title)
			} else {
				fmt.Printf("[%s] %s: %s\n", ev.Phase, ev.Type, ev.Message)
			}
		}
	}

	_ = cmd.Wait()
}
```
