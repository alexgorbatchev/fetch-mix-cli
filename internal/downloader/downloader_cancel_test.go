//go:build unix

package downloader

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/proctest"
	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

// Cancelling a download must let fetch-track and the tools it started stop
// on SIGTERM instead of being killed outright.
func TestDownloadSetCancellationTerminatesFetchTrackGracefully(t *testing.T) {
	t.Setenv("AGENT", "1")
	fixture := proctest.NewGroupFixture(t, false)
	oldCmd := fetchTrackCmdName
	oldDelay := trackDownloadDelay
	fetchTrackCmdName = fixture.Command
	trackDownloadDelay = 0
	t.Cleanup(func() {
		fetchTrackCmdName = oldCmd
		trackDownloadDelay = oldDelay
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := DownloadOptions{
		Output:    io.Discard,
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

	if got := fixture.Terminated(t); !slices.Equal(got, proctest.Members) {
		t.Fatalf("processes that received SIGTERM = %v, want %v", got, proctest.Members)
	}
	fixture.WaitGone(t, 5*time.Second)
}
