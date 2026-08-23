package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
	"github.com/zendev-sh/goai"
	"github.com/zendev-sh/goai/provider"
	"github.com/zendev-sh/goai/provider/anthropic"
	"github.com/zendev-sh/goai/provider/compat"
	"github.com/zendev-sh/goai/provider/deepseek"
	"github.com/zendev-sh/goai/provider/google"
	"github.com/zendev-sh/goai/provider/groq"
	"github.com/zendev-sh/goai/provider/ollama"
	"github.com/zendev-sh/goai/provider/openai"
	"github.com/zendev-sh/goai/provider/openrouter"
)

type ProviderInfo struct {
	ID           string
	Name         string
	DefaultModel string
	EnvVar       string
	Status       string
	Active       bool
	IsDefault    bool
}

// SupportedProviders defines supported LLM providers in auto-detection priority order.
// Priority: Ollama (1st) -> LiteLLM (2nd) -> Gemini (3rd) -> OpenAI (4th) -> Anthropic -> OpenRouter -> DeepSeek -> Groq -> Custom
var SupportedProviders = []ProviderInfo{
	{ID: "ollama", Name: "Ollama (Local)", DefaultModel: "llama3.2", EnvVar: "OLLAMA_HOST"},
	{ID: "litellm", Name: "LiteLLM (Proxy)", DefaultModel: "gemini-2.5-flash", EnvVar: "LITELLM_BASE_URL"},
	{ID: "gemini", Name: "Google Gemini", DefaultModel: "gemini-2.5-flash", EnvVar: "GEMINI_API_KEY"},
	{ID: "openai", Name: "OpenAI", DefaultModel: "gpt-4o-mini", EnvVar: "OPENAI_API_KEY"},
	{ID: "anthropic", Name: "Anthropic Claude", DefaultModel: "claude-3-5-haiku-latest", EnvVar: "ANTHROPIC_API_KEY"},
	{ID: "openrouter", Name: "OpenRouter", DefaultModel: "google/gemini-2.5-flash", EnvVar: "OPENROUTER_API_KEY"},
	{ID: "deepseek", Name: "DeepSeek", DefaultModel: "deepseek-chat", EnvVar: "DEEPSEEK_API_KEY"},
	{ID: "groq", Name: "Groq", DefaultModel: "llama-3.3-70b-versatile", EnvVar: "GROQ_API_KEY"},
	{ID: "custom", Name: "Custom OpenAI-Compat", DefaultModel: "gpt-4o-mini", EnvVar: "OPENAI_BASE_URL"},
}

// GetProviderStatuses checks active environment variables and returns current status for all providers.
// WILL BE USED: Selected as default under auto-detection.
// Detected: API key/endpoint present, but lower in auto-detection priority.
// Not Detected: No API key or endpoint configured.
func GetProviderStatuses() []ProviderInfo {
	result := make([]ProviderInfo, len(SupportedProviders))
	var defaultFound bool

	for i, p := range SupportedProviders {
		val := strings.TrimSpace(os.Getenv(p.EnvVar))
		switch p.ID {
		case "ollama":
			if m := strings.TrimSpace(os.Getenv("OLLAMA_MODEL")); m != "" {
				p.DefaultModel = m
			}
			p.Active = val != "" || strings.TrimSpace(os.Getenv("OLLAMA_URL")) != ""
			if p.Active {
				if !defaultFound {
					p.IsDefault = true
					p.Status = "WILL BE USED"
					defaultFound = true
				} else {
					p.Status = "Detected"
				}
			} else {
				p.Status = "Not Detected (default: http://localhost:11434)"
			}
		case "litellm":
			if m := strings.TrimSpace(os.Getenv("LITELLM_MODEL")); m != "" {
				p.DefaultModel = m
			}
			p.Active = val != "" || strings.TrimSpace(os.Getenv("LITELLM_URL")) != "" || strings.TrimSpace(os.Getenv("LITELLM_API_KEY")) != ""
			if p.Active {
				if !defaultFound {
					p.IsDefault = true
					p.Status = "WILL BE USED"
					defaultFound = true
				} else {
					p.Status = "Detected"
				}
			} else {
				p.Status = "Not Detected (default: http://localhost:4000)"
			}
		default:
			p.Active = val != ""
			if p.Active {
				if !defaultFound {
					p.IsDefault = true
					p.Status = "WILL BE USED"
					defaultFound = true
				} else {
					p.Status = "Detected"
				}
			} else {
				p.Status = "Not Detected"
			}
		}
		result[i] = p
	}
	return result
}

// ResolveProvider determines the active provider ID and model name.
func ResolveProvider(requestedProvider, requestedModel string) (string, string, error) {
	pID := strings.ToLower(strings.TrimSpace(requestedProvider))
	if pID == "" || pID == "auto" {
		for _, p := range GetProviderStatuses() {
			if p.Active {
				m := requestedModel
				if m == "" {
					m = p.DefaultModel
				}
				return p.ID, m, nil
			}
		}
		return "", "", fmt.Errorf("no active LLM API keys or endpoints detected in environment. Set OLLAMA_HOST, LITELLM_BASE_URL, GEMINI_API_KEY, OPENAI_API_KEY, or run 'fetch-mix ai' for details")
	}

	for _, p := range GetProviderStatuses() {
		if p.ID == pID {
			m := requestedModel
			if m == "" {
				m = p.DefaultModel
			}
			return p.ID, m, nil
		}
	}

	return "", "", fmt.Errorf("unsupported provider %q. Run 'fetch-mix ai' to view supported providers", pID)
}

type GeminiTrack struct {
	Artist    string `json:"artist"`
	Title     string `json:"title"`
	Timestamp string `json:"timestamp,omitempty"`
}

type GeminiResult struct {
	Found        bool                `json:"found"`
	CommentID    string              `json:"commentId,omitempty"`
	Tracks       []GeminiTrack       `json:"tracks,omitempty"`
	SkippedItems []types.SkippedItem `json:"skippedItems,omitempty"`
}

// ExtractTracklistWithAI queries the configured provider via goai for tracklist extraction.
func ExtractTracklistWithAI(ctx context.Context, providerID, modelName string, prompt string) (*GeminiResult, error) {
	resolvedProvider, modelName, err := ResolveProvider(providerID, modelName)
	if err != nil {
		return nil, err
	}

	var model provider.LanguageModel

	switch resolvedProvider {
	case "ollama":
		host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
		if host == "" {
			host = strings.TrimSpace(os.Getenv("OLLAMA_URL"))
		}
		if host != "" {
			model = ollama.Chat(modelName, ollama.WithBaseURL(host))
		} else {
			model = ollama.Chat(modelName)
		}
	case "litellm":
		baseURL := strings.TrimSpace(os.Getenv("LITELLM_BASE_URL"))
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("LITELLM_URL"))
		}
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("LITELLM_API_BASE"))
		}
		if baseURL == "" {
			baseURL = "http://localhost:4000"
		}
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimRight(baseURL, "/") + "/v1"
		}
		key := strings.TrimSpace(os.Getenv("LITELLM_API_KEY"))
		if key == "" {
			key = strings.TrimSpace(os.Getenv("LITELLM_KEY"))
		}
		var opts []compat.Option
		opts = append(opts, compat.WithBaseURL(baseURL))
		if key != "" {
			opts = append(opts, compat.WithAPIKey(key))
		}
		model = compat.Chat(modelName, opts...)
	case "gemini":
		key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		if key != "" {
			model = google.Chat(modelName, google.WithAPIKey(key))
		} else {
			model = google.Chat(modelName)
		}
	case "openai":
		key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if key != "" {
			model = openai.Chat(modelName, openai.WithAPIKey(key))
		} else {
			model = openai.Chat(modelName)
		}
	case "anthropic":
		key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		if key != "" {
			model = anthropic.Chat(modelName, anthropic.WithAPIKey(key))
		} else {
			model = anthropic.Chat(modelName)
		}
	case "openrouter":
		key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
		if key != "" {
			model = openrouter.Chat(modelName, openrouter.WithAPIKey(key))
		} else {
			model = openrouter.Chat(modelName)
		}
	case "deepseek":
		key := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
		if key != "" {
			model = deepseek.Chat(modelName, deepseek.WithAPIKey(key))
		} else {
			model = deepseek.Chat(modelName)
		}
	case "groq":
		key := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
		if key != "" {
			model = groq.Chat(modelName, groq.WithAPIKey(key))
		} else {
			model = groq.Chat(modelName)
		}
	case "custom":
		baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		if baseURL == "" {
			return nil, fmt.Errorf("OPENAI_BASE_URL environment variable is not set")
		}
		key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		var opts []compat.Option
		opts = append(opts, compat.WithBaseURL(baseURL))
		if key != "" {
			opts = append(opts, compat.WithAPIKey(key))
		}
		model = compat.Chat(modelName, opts...)
	default:
		return nil, fmt.Errorf("unsupported provider %q", resolvedProvider)
	}

	res, err := goai.GenerateObject[GeminiResult](ctx, model, goai.WithPrompt(prompt))
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed (%s/%s): %w", resolvedProvider, modelName, err)
	}

	return &res.Object, nil
}
