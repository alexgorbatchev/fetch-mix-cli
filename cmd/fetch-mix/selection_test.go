package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestYouTubeCandidateDiscoveryDoesNotExtractOrDownload(t *testing.T) {
	for _, prefix := range [][]string{nil, {"youtube"}} {
		t.Run(strings.Join(prefix, " "), func(t *testing.T) {
			command := newRootCmd()
			var output bytes.Buffer
			command.SetOut(&output)
			args := append(append([]string{}, prefix...), "--json", "--list-candidates", "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			var result mixCandidateResult
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Candidates) != 1 || result.Candidates[0].URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestExplicitCandidatePreviewScrapesExactSelectedSource(t *testing.T) {
	llm := setupMockLLMServer(t)
	defer llm.Close()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/1001tracklists.com/selected-set.html" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("<html><p>00:00 Artist - Track</p></html>"))
	}))
	defer server.Close()
	selected := server.URL + "/1001tracklists.com/selected-set.html"
	command := newRootCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--json", "--dry-run", "--no-cache", "--llm-provider", "openai", "--llm-model", "test-model", "--candidate-url", selected, "original query"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result mixResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "preview" || result.SourceURL != selected || len(result.Tracks) != 1 || requests != 1 {
		t.Fatalf("result=%+v requests=%d", result, requests)
	}
}

func TestCandidateURLsAreValidatedBeforeNetworkWork(t *testing.T) {
	for _, selected := range []string{"file:///etc/passwd", "https://user:secret@example.org/set", "not a URL"} {
		command := newRootCmd()
		command.SetArgs([]string{"--json", "--list-candidates", "--candidate-url", selected, "query"})
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted %q", selected)
		}
	}
	command := newRootCmd()
	command.SetArgs([]string{"youtube", "--json", "--list-candidates", "not a video"})
	if err := command.Execute(); err == nil {
		t.Fatal("accepted invalid YouTube source")
	}
}

func TestExplicitCandidateURLDoesNotSearchAgain(t *testing.T) {
	command := newRootCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	selected := "https://www.1001tracklists.com/tracklist/abc/selected-set.html"
	command.SetArgs([]string{"--json", "--list-candidates", "--candidate-url", selected, "original artist query"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result mixCandidateResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].URL != selected {
		t.Fatalf("result=%+v", result)
	}
}
