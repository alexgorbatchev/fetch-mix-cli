package display

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTextAndTerminalOutput(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	cases := []struct{ input, want string }{
		{home, "~"}, {home + "/music", "~/music"},
		{"open " + home + "/music: missing", "open ~/music: missing"},
		{"path=" + home + "; other=" + home + "/file", "path=~; other=~/file"},
		{home + "other/file", home + "other/file"},
		{"/prefix" + home + "/file", "/prefix" + home + "/file"},
		{"unrelated", "unrelated"},
	}
	for _, tc := range cases {
		if got := Text(tc.input); got != tc.want {
			t.Errorf("Text(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
	var out bytes.Buffer
	if _, err := Fprintf(&out, "open %s\n", home+"/file"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "open ~/file\n" {
		t.Fatal(out.String())
	}
	f, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	if _, err := Printf("%s\n", home+"/one"); err != nil {
		t.Fatal(err)
	}
	if _, err := Println(home + "/two"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "~/one\n~/two\n" {
		t.Fatal(string(data))
	}
	if _, err := Fprintf(f, "%s", home); err == nil {
		t.Fatal("lost write error")
	}
	t.Setenv("HOME", "")
	if got := Path(home); got != home {
		t.Fatal(got)
	}
	if got := Text(home); got != home {
		t.Fatal(got)
	}
}
