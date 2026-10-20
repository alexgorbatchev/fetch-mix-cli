package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/downloader"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestJSONPipelineFailures(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	original := deps.RequiredDependencies
	t.Cleanup(func() { deps.RequiredDependencies = original })
	deps.RequiredDependencies = []deps.Dependency{{Name: "missing-fixture", MinVersion: "1.0.0"}}
	for _, args := range [][]string{
		{"--json"}, {"youtube", "--json"},
		{"--json", "--candidate-url=invalid", "query"},
		{"youtube", "--json", "--candidate-url=invalid", "query"},
		{"--json", "--progress-target=unsupported://fixture", "query"},
		{"youtube", "--json", "--progress-target=unsupported://fixture", "query"},
		{"--json", "query"}, {"youtube", "--json", "query"},
		{"youtube", "--json", "--list-candidates", "invalid"},
		{"ai", "inspect", "--json"}, {"ai", "inspect", "--json", "missing-fixture"},
	} {
		t.Run(args[0]+"/"+args[len(args)-1], func(t *testing.T) {
			command := newRootCmd()
			var out bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(args)
			err := command.Execute()
			var result struct{ Status, Error string }
			if decodeErr := json.Unmarshal(out.Bytes(), &result); err == nil || decodeErr != nil || result.Status != "error" || result.Error == "" {
				t.Fatalf("failure result lost: %q: %v (JSON: %v)", out.String(), err, decodeErr)
			}
		})
	}
}

func TestJSONDownloadFailureRemainsOneResult(t *testing.T) {
	newRootCmd()
	jsonOutput = true
	var out bytes.Buffer
	resultWriter = &out
	t.Cleanup(func() { jsonOutput = false; resultWriter = os.Stdout })
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	err := finishMix(context.Background(), downloader.DownloadOptions{MixTitle: "Set", OutputDir: blocked, Tracks: []types.Track{{Artist: "Artist", Title: "Track"}}}, "https://example.com/set")
	if err = writeJSONFailure(err); err == nil {
		t.Fatal("download failure lost")
	}
	var result mixResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "error" || result.SourceURL != "https://example.com/set" {
		t.Fatalf("download envelope duplicated/lost: %q %v", out.String(), err)
	}
	resultWriter = failedResultWriter{}
	if err := writeJSONFailure(errors.New("operation failed")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write failure lost: %v", err)
	}
}

type failedResultWriter struct{}

func (failedResultWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
