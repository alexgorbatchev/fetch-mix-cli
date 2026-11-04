//go:build unix

package youtube

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/proctest"
)

// A cancelled yt-dlp run must get SIGTERM across its process group first, so
// it can remove its temporary files, and SIGKILL only once the grace period
// has passed.
func TestFetchVideoCommentsCancellationTerminatesProcessGroup(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)

	// Ignoring SIGTERM means only SIGKILL can end the fake yt-dlp.
	fixture := proctest.NewGroupFixture(t, true)
	oldYtDlp := ytdlpCmdName
	ytdlpCmdName = fixture.Command
	t.Cleanup(func() { ytdlpCmdName = oldYtDlp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := FetchVideoComments(ctx, "https://www.youtube.com/watch?v=11111111111", true)
		done <- err
	}()

	fixture.WaitReady(t)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("FetchVideoComments() error = %v, want context.Canceled", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancelled yt-dlp was never killed")
	}

	if got := fixture.Terminated(t); !slices.Equal(got, proctest.Members) {
		t.Fatalf("processes that received SIGTERM = %v, want %v", got, proctest.Members)
	}
	fixture.WaitGone(t, 5*time.Second)
}
