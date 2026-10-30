package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/godeps"
)

// Exercise the real manager and subprocess boundary with local executables.
func localDependencies(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(".tmp", 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FETCH_MIX_DEV", "1")
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("PATH", dir)
	old := deps.RequiredDependencies
	t.Cleanup(func() { deps.RequiredDependencies = old })
	deps.RequiredDependencies = append([]deps.Dependency(nil), old...)
	for i := range deps.RequiredDependencies {
		dep := &deps.RequiredDependencies[i]
		name := dep.Name
		dep.Installer = godeps.InstallerFunc(func(ctx context.Context, target string) error {
			return os.WriteFile(filepath.Join(target, name), []byte("#!/bin/sh\nprintf '9999.0.0\\n'\n"), 0755)
		})
	}
}

func TestDependencyCommandsLocal(t *testing.T) {
	for _, mode := range []string{"0", "1"} {
		for _, args := range [][]string{{"deps", "install"}, {"deps", "install", "fetch-track"}, {"deps", "update"}, {"deps", "update", "yt-dlp"}} {
			t.Run(mode+strings.Join(args, "_"), func(t *testing.T) {
				localDependencies(t)
				t.Setenv("AGENT", mode)
				cmd := newRootCmd()
				cmd.SetArgs(args)
				out := captureCLIStdout(t, func() {
					if err := cmd.ExecuteContext(context.Background()); err != nil {
						t.Fatal(err)
					}
				})
				if !strings.Contains(out, "installed") && !strings.Contains(out, "updated") {
					t.Fatalf("missing result: %s", out)
				}
				dir, err := deps.GetManagedBinDir()
				if err != nil {
					t.Fatal(err)
				}
				name := "fetch-track"
				if len(args) > 2 {
					name = args[2]
				}
				if info, err := os.Stat(filepath.Join(dir, name)); err != nil || info.Mode()&0111 == 0 {
					t.Fatalf("executable missing: %v", err)
				}
			})
		}
	}
}

func TestProviderPresentation(t *testing.T) {
	for _, mode := range []string{"0", "1"} {
		for _, args := range [][]string{{"ai", "list"}, {"ai", "inspect", " OpenAI "}} {
			t.Run(mode+strings.Join(args, "_"), func(t *testing.T) {
				t.Setenv("AGENT", mode)
				cmd := newRootCmd()
				cmd.SetArgs(args)
				out := captureCLIStdout(t, func() {
					if err := cmd.Execute(); err != nil {
						t.Fatal(err)
					}
				})
				for _, want := range []string{"openai", "OPENAI_API_KEY", "gpt-4o-mini"} {
					if !strings.Contains(out, want) {
						t.Errorf("missing %q in %s", want, out)
					}
				}
				if mode == "0" && !strings.Contains(out, "Model") {
					t.Errorf("missing human labels: %s", out)
				}
			})
		}
	}
}

func TestPipelinesDownloadAndReport(t *testing.T) {
	for _, pipeline := range []string{"youtube", "web", "failure", "playlist failure"} {
		t.Run(pipeline, func(t *testing.T) {
			server := setupMockLLMServer(t)
			defer server.Close()
			localDependencies(t)
			t.Setenv("AGENT", "0")
			c, err := cache.New()
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"fetch-track", "yt-dlp", "ffmpeg"} {
				if err := c.Put("godeps_"+name+".json", "9999.0.0"); err != nil {
					t.Fatal(err)
				}
			}
			if err := c.Put("yt_tracks_abcdefghijk.json", map[string]any{"videoTitle": "Fixture Mix", "tracks": []map[string]string{{"artist": "First", "title": "One", "timestamp": "01:05:00"}, {"artist": "Second", "title": "Two"}}}); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "out")
			if err := os.MkdirAll(out, 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FETCH_MIX_TEST_OUTPUT", filepath.Join(out, "Fixture.m4a"))
			bin := filepath.Join(strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))[0], "fetch-track")
			script := "#!/bin/sh\nprintf audio > \"$FETCH_MIX_TEST_OUTPUT\"\n"
			if pipeline == "playlist failure" {
				if err := os.Mkdir(filepath.Join(out, "playlist.m3u"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("AGENT", "1")
			}
			if pipeline == "failure" {
				script = "#!/bin/sh\nexit 1\n"
				t.Setenv("AGENT", "1")
			}
			if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			cmd := newRootCmd()
			query := "abcdefghijk"
			if pipeline == "web" {
				query = server.URL + "/set"
			}
			cmd.SetArgs([]string{query, "-o", out, "-p", "custom", "--progress-target", "stdout"})
			output := captureCLIStdout(t, func() {
				if pipeline == "failure" || pipeline == "playlist failure" {
					old := os.Args
					defer func() { os.Args = old }()
					os.Args = []string{"fetch-mix", query, "-o", out, "--progress-target", "stdout"}
					if code := runMain(context.Background()); code != 1 {
						t.Fatalf("exit=%d, want 1", code)
					}
					return
				}
				if err := cmd.ExecuteContext(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
			if pipeline == "playlist failure" {
				for _, want := range []string{"status: error", `"downloaded":2`, `"failed":0`, `"error":"generating playlist:`} {
					if !strings.Contains(output, want) {
						t.Fatalf("missing %q: %s", want, output)
					}
				}
				return
			}
			if pipeline == "failure" {
				for _, want := range []string{"status: error", `"failed":2`, `"error":"2 of 2 tracks failed`} {
					if !strings.Contains(output, want) {
						t.Fatalf("missing %q: %s", want, output)
					}
				}
				data, err := os.ReadFile(filepath.Join(out, "mix_manifest.json"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(string(data), `"status": "failed"`) != 2 {
					t.Fatalf("partial results not saved: %s", data)
				}
				return
			}
			for _, name := range []string{"01 - Fixture.m4a", "02 - Fixture.m4a", "playlist.m3u", "mix_manifest.json"} {
				if _, err := os.Stat(filepath.Join(out, name)); err != nil {
					t.Fatal(err)
				}
			}
			for _, want := range []string{`"phase":"parse"`, `"downloaded":2`, "[01:05:00]"} {
				if pipeline == "web" && want == "[01:05:00]" {
					continue
				}
				if !strings.Contains(output, want) {
					t.Errorf("missing %s: %s", want, output)
				}
			}
			playlist, err := os.ReadFile(filepath.Join(out, "playlist.m3u"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(playlist), "01 - Fixture.m4a") {
				t.Fatalf("playlist: %s", playlist)
			}
		})
	}
}

func TestRunMainUsageError(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"fetch-mix", "--invalid-option"}
	if got := runMain(context.Background()); got != 1 {
		t.Fatalf("exit=%d, want 1", got)
	}
}
