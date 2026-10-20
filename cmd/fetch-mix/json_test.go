package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/downloader"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
	"os"
	"testing"
)

func TestAIListJSON(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"ai", "list", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Providers []struct{ ID string } `json:"providers"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("JSON: %v, output %s", err, output.String())
	}
	if len(result.Providers) == 0 {
		t.Fatal("missing provider descriptions")
	}
}

func TestMixJSONPreview(t *testing.T) {
	newRootCmd()
	jsonOutput = true
	var output bytes.Buffer
	resultWriter = &output
	t.Cleanup(func() { jsonOutput = false; resultWriter = os.Stdout })
	dir := t.TempDir()
	err := finishMix(context.Background(), downloader.DownloadOptions{MixTitle: "Set", OutputDir: dir, DryRun: true, Tracks: []types.Track{{Artist: "Artist", Title: "Track", Timestamp: "00:12"}}}, "https://example.com/set")
	if err != nil {
		t.Fatal(err)
	}
	var result mixResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "preview" || result.SourceURL != "https://example.com/set" || len(result.Tracks) != 1 || result.Tracks[0].Title != "Track" || result.Tracks[0].Timestamp != "00:12" || result.Tracks[0].OutputFile != "" {
		t.Fatalf("result: %+v", result)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview wrote files: %v %v", entries, err)
	}
}
