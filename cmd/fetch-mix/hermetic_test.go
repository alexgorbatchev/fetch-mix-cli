package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runHermeticTests(m))
}

func runHermeticTests(m *testing.M) int {
	tempRoot := filepath.Join("..", "..", ".tmp")
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := os.MkdirTemp(tempRoot, "cli-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	previous, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.Chdir(previous)
	if err := os.Setenv("FETCH_MIX_DEV", "0"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Installed media tools and managed binaries must never be invoked by unit tests.
	for _, key := range []string{"PATH", "HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		if err := os.Setenv(key, dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	http.DefaultTransport = localTransport{http.DefaultTransport}
	return m.Run()
}

type localTransport struct{ http.RoundTripper }

func (t localTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	host := r.URL.Hostname()
	if host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return nil, fmt.Errorf("external request forbidden in unit tests: %s", r.URL)
	}
	return t.RoundTripper.RoundTrip(r)
}
