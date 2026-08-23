package deps

import (
	"os"
	"testing"
)

func TestIsAgentMode(t *testing.T) {
	os.Unsetenv("AGENT")
	if IsAgentMode() {
		t.Errorf("IsAgentMode() = true, want false when AGENT is unset")
	}

	os.Setenv("AGENT", "1")
	if !IsAgentMode() {
		t.Errorf("IsAgentMode() = false, want true when AGENT=1")
	}

	os.Setenv("AGENT", "true")
	if !IsAgentMode() {
		t.Errorf("IsAgentMode() = false, want true when AGENT=true")
	}

	os.Setenv("AGENT", "0")
	if IsAgentMode() {
		t.Errorf("IsAgentMode() = true, want false when AGENT=0")
	}

	os.Unsetenv("AGENT")
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
		{"fetch-track", "v1.2.3\n", "1.2.3"},
		{"firecrawl", "1.0.0", "1.0.0"},
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
		{"1.2.0", "1.1.0", false},
		{"1.0.0", "1.1.0", true},
	}

	for _, tt := range tests {
		err := CompareVersions(tt.actual, tt.min)
		if (err != nil) != tt.wantErr {
			t.Errorf("CompareVersions(%q, %q) error = %v, wantErr %v", tt.actual, tt.min, err, tt.wantErr)
		}
	}
}
