package main

import (
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree"
	"github.com/spf13/cobra"
)

var techCatalog = cobrahelptree.TechCatalog{
	"fetch-mix": {
		Summary:     "Fetch, extract, and download entire DJ mix tracklists using fetch-track CLI",
		Description: "Searches MixesDB, crawlable web mirrors, 1001tracklists, or YouTube comments (via multi-provider LLMs), parses tracklists deterministically, and downloads individual tracks using fetch-track CLI.",
		Args:        "[url|query]",
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
		Args:        "<provider>",
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
		Args:        "[dep...]",
	},
	"fetch-mix dependencies update": {
		Summary:     "Update external dependencies to latest versions",
		Description: "Fetches and upgrades all or specified external CLI dependencies to latest releases.",
		Args:        "[dep...]",
	},
	"fetch-mix youtube": {
		Summary:     "Extract tracklist from YouTube comments and download",
		Description: "Fetches top comments from YouTube video via yt-dlp, filters candidates, parses tracklists via LLM, and downloads audio.",
		Args:        "[url|id]",
	},
	"fetch-mix upgrade": {
		Summary:     "Upgrade fetch-mix binary to latest release",
		Description: "Checks GitHub releases and replaces local executable with latest released binary without requiring package managers.",
	},
}

func setupHelp(cmd *cobra.Command) {
	cobrahelptree.Setup(cmd, cobrahelptree.TreeOptions{
		TechCatalog: techCatalog,
	})
}
