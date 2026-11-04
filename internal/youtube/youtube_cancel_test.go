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
// has passed. FetchVideoComments must not return while any of the group is
// still running, since fetch-mix exits right after a cancelled run.
func TestFetchVideoCommentsCancellationTerminatesProcessGroup(t *testing.T) {
	tests := []struct {
		name      string
		survivors proctest.Survivors
	}{
		{name: "every member ignores SIGTERM", survivors: proctest.Survivors{Parent: true, Child: true}},
		{name: "yt-dlp exits but its subprocess ignores SIGTERM", survivors: proctest.Survivors{Child: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Chdir(tempDir)
			t.Setenv("FETCH_MIX_DEV", "0")
			t.Setenv("XDG_CACHE_HOME", tempDir)

			fixture := proctest.NewGroupFixture(t, tt.survivors)
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
			// SIGKILL was sent before the return; allow only for its delivery.
			fixture.WaitGone(t, proctest.KillDelivery)

			if got := fixture.Terminated(t); !slices.Equal(got, proctest.Members) {
				t.Fatalf("processes that received SIGTERM = %v, want %v", got, proctest.Members)
			}
		})
	}
}
