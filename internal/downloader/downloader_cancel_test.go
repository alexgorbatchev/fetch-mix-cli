//go:build unix

package downloader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/proctest"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

// Cancelling a download must let fetch-track and the tools it started stop
// on SIGTERM instead of being killed outright, and DownloadSet must not return
// while a tool that ignored SIGTERM is still running, since fetch-mix exits
// right after a cancelled download.
func TestDownloadSetCancellationStopsFetchTrackGroup(t *testing.T) {
	t.Setenv("AGENT", "1")
	fixture := proctest.NewGroupFixture(t, proctest.Survivors{Child: true})
	oldCmd := fetchTrackCmdName
	oldDelay := trackDownloadDelay
	fetchTrackCmdName = fixture.Command
	trackDownloadDelay = 0
	t.Cleanup(func() {
		fetchTrackCmdName = oldCmd
		trackDownloadDelay = oldDelay
	})

	// Agent mode hands fetch-track this file directly, as it does os.Stdout,
	// so no output pipe keeps DownloadSet waiting for the process group.
	output, err := os.Create(filepath.Join(t.TempDir(), "output"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := DownloadOptions{
		Output:    output,
		MixTitle:  "Cancelled Set",
		OutputDir: filepath.Join(t.TempDir(), "downloads"),
		Tracks:    []types.Track{{Artist: "Artist", Title: "Track", RawString: "Artist - Track"}},
	}
	done := make(chan error, 1)
	go func() { done <- DownloadSet(ctx, opts) }()

	fixture.WaitReady(t)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DownloadSet() error = %v, want context.Canceled", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancelled fetch-track was never stopped")
	}

	// SIGKILL was sent before the return; allow only for its delivery.
	fixture.WaitGone(t, proctest.KillDelivery)
	if got := fixture.Terminated(t); !slices.Equal(got, proctest.Members) {
		t.Fatalf("processes that received SIGTERM = %v, want %v", got, proctest.Members)
	}
}
