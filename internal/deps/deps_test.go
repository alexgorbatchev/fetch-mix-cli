package deps

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

func TestDependencyReport_Summary(t *testing.T) {
	tests := []struct {
		name   string
		report DependencyReport
		want   string
	}{
		{
			name: "satisfied",
			report: DependencyReport{
				Name:            "fetch-track",
				MinVersion:      "1.4.0",
				DetectedVersion: "1.4.0",
				Installed:       true,
				Satisfied:       true,
			},
			want: "fetch-track (version 1.4.0, min 1.4.0)",
		},
		{
			name: "installed but outdated",
			report: DependencyReport{
				Name:            "fetch-track",
				MinVersion:      "1.4.0",
				DetectedVersion: "1.3.0",
				Installed:       true,
				Satisfied:       false,
			},
			want: "fetch-track (installed 1.3.0, required >= 1.4.0)",
		},
		{
			name: "not installed",
			report: DependencyReport{
				Name:       "yt-dlp",
				MinVersion: "2024.08.01",
				Installed:  false,
				Satisfied:  false,
			},
			want: "yt-dlp (not installed, required >= 2024.08.01)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.Summary()
			if got != tt.want {
				t.Errorf("DependencyReport.Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		dep   string
		input string
		want  string
	}{
		{"yt-dlp", "2024.08.01", "2024.08.01"},
		{"ffmpeg", "ffmpeg version 6.0 Copyright (c) 2000-2023", "6.0"},
		{"fetch-track", "dev\n", "dev"},
		{"fetch-track", "1.3.0\n", "1.3.0"},
		{"fetch-track", "fetch-track version 1.0.0\n", "1.0.0"},
		{"fetch-track", "v1.2.3\n", "1.2.3"},
		{"other", "1.2.3", "1.2.3"},
	}

	for _, tt := range tests {
		got := ParseVersionOutput(tt.dep, tt.input)
		if got != tt.want {
			t.Errorf("ParseVersionOutput(%q, %q) = %q, want %q", tt.dep, tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		actual  string
		min     string
		wantErr bool
	}{
		{"2024.08.01", "2024.08.01", false},
		{"2024.09.01", "2024.08.01", false},
		{"2024.07.01", "2024.08.01", true},
		{"dev", "0.1.0", false},
		{"N-12345", "1.0.0", false},
		{"git-master", "1.0.0", false},
		{"1.0.0.1", "1.0.0", false},
		{"1.0.0", "1.1.0", true},
		{"", "1.0.0", false},
	}

	for _, tt := range tests {
		err := CompareVersions(tt.actual, tt.min)
		if (err != nil) != tt.wantErr {
			t.Errorf("CompareVersions(%q, %q) error = %v, wantErr %v", tt.actual, tt.min, err, tt.wantErr)
		}
	}
}

func TestVerifyDependenciesWithRunner(t *testing.T) {
	ctx := context.Background()

	// 1. All satisfied
	mockRunnerOk := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "fetch-track":
			return []byte("1.4.0"), nil
		case "yt-dlp":
			return []byte("2025.01.01"), nil
		case "ffmpeg":
			return []byte("ffmpeg version 5.1"), nil
		}
		return nil, errors.New("unknown")
	}

	reports, err := VerifyDependenciesWithRunner(ctx, mockRunnerOk, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	for _, r := range reports {
		if !r.Satisfied {
			t.Errorf("expected %s satisfied, got %#v", r.Name, r)
		}
	}

	// 2. Missing binary in agent mode
	t.Setenv("AGENT", "1")
	mockRunnerMissing := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	reports, err = VerifyDependenciesWithRunner(ctx, mockRunnerMissing, nil)
	if err == nil {
		t.Errorf("expected error for missing binaries")
	}

	// 3. Outdated binary in non-agent mode
	t.Setenv("AGENT", "0")
	mockRunnerOutdated := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0.0.1"), nil
	}
	reports, err = VerifyDependenciesWithRunner(ctx, mockRunnerOutdated, nil)
	if err == nil {
		t.Errorf("expected error for outdated binary")
	}
}

func TestVerifyDependencies_WithCache(t *testing.T) {
	tempDir := t.TempDir()
	c, _ := cache.New(tempDir)

	_ = c.Put("godeps_fetch-track.json", "1.5.0")
	_ = c.Put("godeps_yt-dlp.json", "2025.01.01")
	_ = c.Put("godeps_ffmpeg.json", "ffmpeg version 6.0")

	ctx := context.Background()
	failRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		t.Fatalf("runner should not be called when cache is populated for %s", name)
		return nil, errors.New("fail")
	}

	reports, err := VerifyDependenciesWithRunner(ctx, failRunner, c)
	if err != nil {
		t.Fatalf("unexpected error with cached deps: %v", err)
	}
	if len(reports) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(reports))
	}
}

func TestCheckDependencies_Helpers(t *testing.T) {
	ctx := context.Background()
	okRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "yt-dlp" {
			return []byte("2025.01.01"), nil
		}
		return []byte("10.0.0"), nil
	}

	if err := CheckDependenciesWithRunner(ctx, okRunner, nil); err != nil {
		t.Errorf("CheckDependenciesWithRunner failed: %v", err)
	}

	tempDir := t.TempDir()
	t.Setenv("FETCH_MIX_DEV", "0")
	t.Setenv("XDG_CACHE_HOME", tempDir)
	c, _ := cache.New()
	if c != nil {
		_ = c.Put("godeps_fetch-track.json", "1.5.0")
		_ = c.Put("godeps_yt-dlp.json", "2025.01.01")
		_ = c.Put("godeps_ffmpeg.json", "ffmpeg version 6.0")
	}

	if err := CheckDependencies(ctx, c); err != nil {
		t.Errorf("CheckDependencies with cache failed: %v", err)
	}

	if _, err := VerifyDependencies(ctx, c); err != nil {
		t.Errorf("VerifyDependencies with cache failed: %v", err)
	}
}

func TestDefaultRunner(t *testing.T) {
	ctx := context.Background()
	out, err := DefaultRunner(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("DefaultRunner echo failed: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("unexpected output: %q", string(out))
	}
}
