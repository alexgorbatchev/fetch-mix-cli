package main

import (
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

const skillCommandOutput = "Install the fetch-mix skill using the Skills CLI:\n\n  npx skills add alexgorbatchev/fetch-mix-cli --skill fetch-mix\n\nSkill:\n\n  https://github.com/alexgorbatchev/fetch-mix-cli/tree/main/skills/fetch-mix\n"

var techCatalog = cobrahelptree.TechCatalog{
	"fetch-mix": {
		Summary:     "Fetch, extract, and download entire DJ mix tracklists using fetch-track CLI",
		Description: "Searches MixesDB, crawlable web mirrors, 1001tracklists, or YouTube comments (via multi-provider LLMs), parses tracklists deterministically, and downloads individual tracks using fetch-track CLI.",
		Args:        []cobrahelptree.ArgSpec{{Name: "<url|query>", Description: "DJ set title, tracklist URL, or YouTube URL/video ID"}},
		Env: []cobrahelptree.EnvSpec{
			{Name: "AGENT", Description: "1, true, yes select compact non-interactive output"},
			{Name: "FETCH_MIX_PROGRESS_TARGET", Description: "Progress destination when flags are omitted"},
			{Name: "FETCH_MIX_DEV", Description: "1 or true selects local .tmp caching"},
		},
	},
	"fetch-mix ai": {
		Summary:     "Manage and inspect AI / LLM configuration",
		Description: "Configure, inspect, and test LLM provider connectivity (Ollama, LiteLLM, Gemini, OpenAI, Claude, OpenRouter, DeepSeek, Groq, custom).",
	},
	"fetch-mix ai list": {
		Summary:     "List supported LLM providers and models",
		Description: "Prints supported LLM providers in auto-detection priority order, default model names, required environment variables, and active detection state.",
	},
	"fetch-mix ai inspect": {
		Summary:     "Inspect configuration and status of an LLM provider",
		Description: "Displays detailed configuration, environment variable requirement, default model, and active connection status for target provider.",
		Args:        []cobrahelptree.ArgSpec{{Name: "<provider>", Description: "Provider ID from ai list, such as ollama or openai"}},
	},
	"fetch-mix dependencies": {
		Summary:     "Manage external binary dependencies (fetch-track, yt-dlp, ffmpeg)",
		Description: "Inspects, installs, and updates required external CLI dependencies in system PATH or managed directory.",
	},
	"fetch-mix dependencies verify": {
		Summary:     "Verify required external binary dependencies",
		Description: "Checks installation and minimum version compliance for fetch-track (>=1.4.0), yt-dlp (>=2024.08.01), and ffmpeg (>=4.4).",
	},
	"fetch-mix dependencies install": {
		Summary:     "Install missing external dependencies",
		Description: "Downloads and installs missing CLI dependencies into ~/.local/share/fetch-mix/bin without modifying global system paths.",
		Args:        []cobrahelptree.ArgSpec{{Name: "[dep...]", Description: "fetch-track, yt-dlp, or ffmpeg; omitted selects all"}},
	},
	"fetch-mix dependencies update": {
		Summary:     "Update external dependencies to latest versions",
		Description: "Fetches and upgrades all or specified external CLI dependencies to latest releases.",
		Args:        []cobrahelptree.ArgSpec{{Name: "[dep...]", Description: "fetch-track, yt-dlp, or ffmpeg; omitted selects all"}},
	},
	"fetch-mix youtube": {
		Summary:     "Extract tracklist from YouTube comments and download",
		Description: "Fetches top comments from YouTube video via yt-dlp, filters candidates, parses tracklists via LLM, and downloads audio.",
		Args:        []cobrahelptree.ArgSpec{{Name: "<url|id>", Description: "YouTube video URL or video ID"}},
	},
	"fetch-mix self upgrade": {
		Summary:     "Upgrade fetch-mix binary to latest release",
		Description: "Checks GitHub releases and replaces local executable with latest released binary without requiring package managers.",
	},
}

func setupHelp(cmd *cobra.Command) {
	if err := cobrahelptree.SetupWithOptions(cmd, cobrahelptree.HelpOptions{
		Catalog: techCatalog,
		Tree:    cobrahelptree.TreeOptions{HideGeneratedCommands: true},
		Skill:   skillCommandOutput,
	}); err != nil {
		panic(err)
	}
}
