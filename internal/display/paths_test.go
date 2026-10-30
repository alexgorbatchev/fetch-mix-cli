package display

import (
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "user")
	t.Setenv("HOME", home)
	for _, tc := range []struct{ in, want string }{
		{home, "~"}, {home + "/music/song.flac", "~/music/song.flac"},
		{home + "name/song.flac", home + "name/song.flac"},
		{"/outside/song.flac", "/outside/song.flac"},
	} {
		if got := Path(tc.in); got != tc.want {
			t.Errorf("Path(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
