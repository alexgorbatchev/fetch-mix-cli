package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProviderStatuses(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test_key")
	t.Setenv("OLLAMA_MODEL", "my-ollama-model")
	t.Setenv("LITELLM_MODEL", "my-litellm-model")
	t.Setenv("OLLAMA_URL", "http://localhost:11434")
	t.Setenv("LITELLM_URL", "http://localhost:4000")

	statuses := GetProviderStatuses()
	foundGemini := false
	foundLiteLLM := false
	foundOllama := false

	for _, s := range statuses {
		if s.ID == "gemini" {
			foundGemini = true
			if !s.Active {
				t.Errorf("Expected Gemini provider to be active")
			}
		}
		if s.ID == "litellm" {
			foundLiteLLM = true
			if s.DefaultModel != "my-litellm-model" {
				t.Errorf("expected litellm model 'my-litellm-model', got %q", s.DefaultModel)
			}
		}
		if s.ID == "ollama" {
			foundOllama = true
			if s.DefaultModel != "my-ollama-model" {
				t.Errorf("expected ollama model 'my-ollama-model', got %q", s.DefaultModel)
			}
		}
	}
	if !foundGemini {
		t.Errorf("Gemini provider missing in GetProviderStatuses")
	}
	if !foundLiteLLM {
		t.Errorf("LiteLLM provider missing in GetProviderStatuses")
	}
	if !foundOllama {
		t.Errorf("Ollama provider missing in GetProviderStatuses")
	}
}

func cleanAllLLMEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"OLLAMA_HOST", "OLLAMA_URL", "OLLAMA_MODEL",
		"LITELLM_BASE_URL", "LITELLM_URL", "LITELLM_API_KEY", "LITELLM_KEY", "LITELLM_API_BASE", "LITELLM_MODEL",
		"GEMINI_API_KEY",
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"ANTHROPIC_API_KEY",
		"OPENROUTER_API_KEY",
		"DEEPSEEK_API_KEY",
		"GROQ_API_KEY",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
}

func TestResolveProviderPriority(t *testing.T) {
	cleanAllLLMEnv(t)

	// No keys -> error
	_, _, err := ResolveProvider("auto", "")
	if err == nil {
		t.Errorf("expected error when no API keys are present")
	}

	// 1. LiteLLM active, Gemini active, OpenAI active -> LiteLLM wins over Gemini/OpenAI
	t.Setenv("LITELLM_BASE_URL", "https://litellm.example.com")
	t.Setenv("GEMINI_API_KEY", "gemini_key")
	t.Setenv("OPENAI_API_KEY", "openai_key")

	pID, model, err := ResolveProvider("auto", "")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if pID != "litellm" {
		t.Errorf("Expected 'litellm' 2nd priority when ollama not set, got %q", pID)
	}
	if model != "gemini-2.5-flash" {
		t.Errorf("Expected 'gemini-2.5-flash', got %q", model)
	}

	// 2. Ollama active -> Ollama wins 1st priority over LiteLLM
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	pID, model, err = ResolveProvider("auto", "")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if pID != "ollama" {
		t.Errorf("Expected 'ollama' 1st priority, got %q", pID)
	}
	if model != "llama3.2" {
		t.Errorf("Expected 'llama3.2', got %q", model)
	}
}

func TestResolveProviderExplicit(t *testing.T) {
	pID, model, err := ResolveProvider("litellm", "gpt-4o")
	if err != nil {
		t.Fatalf("ResolveProvider explicit returned error: %v", err)
	}

	if pID != "litellm" {
		t.Errorf("Expected 'litellm', got %q", pID)
	}
	if model != "gpt-4o" {
		t.Errorf("Expected 'gpt-4o', got %q", model)
	}

	// Unknown provider
	_, _, err = ResolveProvider("non-existent-provider", "")
	if err == nil {
		t.Errorf("expected error for unsupported provider")
	}
}

func TestExtractTracklistWithAI_CustomOpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"found\": true, \"tracks\": [{\"artist\": \"Bicep\", \"title\": \"Glue\", \"timestamp\": \"00:00\"}], \"skippedItems\": []}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test_key")

	ctx := context.Background()
	res, err := ExtractTracklistWithAI(ctx, "custom", "gpt-4o-mini", "extract tracks")
	if err != nil {
		t.Fatalf("ExtractTracklistWithAI failed: %v", err)
	}

	if !res.Found || len(res.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %#v", res)
	}
	if res.Tracks[0].Artist != "Bicep" || res.Tracks[0].Title != "Glue" {
		t.Errorf("unexpected track: %#v", res.Tracks[0])
	}
}

func TestExtractTracklistFromContentWithAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"found\": true, \"tracks\": [{\"artist\": \"Bicep\", \"title\": \"Opal\", \"timestamp\": \"05:00\"}], \"skippedItems\": []}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test_key")

	ctx := context.Background()
	res, err := ExtractTracklistFromContentWithAI(ctx, "custom", "gpt-4o-mini", "1. Bicep - Opal", "Bicep Set")
	if err != nil {
		t.Fatalf("ExtractTracklistFromContentWithAI failed: %v", err)
	}

	if len(res.Tracks) != 1 || res.Tracks[0].Title != "Opal" {
		t.Errorf("unexpected extracted tracks: %#v", res)
	}
}

func TestInitProviderModel_All(t *testing.T) {
	providers := []string{"ollama", "litellm", "gemini", "openai", "anthropic", "openrouter", "deepseek", "groq", "custom"}

	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	t.Setenv("LITELLM_BASE_URL", "http://localhost:4000")
	t.Setenv("LITELLM_API_KEY", "litellm_key")
	t.Setenv("GEMINI_API_KEY", "gemini_key")
	t.Setenv("OPENAI_API_KEY", "openai_key")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic_key")
	t.Setenv("OPENROUTER_API_KEY", "openrouter_key")
	t.Setenv("DEEPSEEK_API_KEY", "deepseek_key")
	t.Setenv("GROQ_API_KEY", "groq_key")
	t.Setenv("OPENAI_BASE_URL", "http://localhost:8000/v1")

	for _, p := range providers {
		model, err := InitProviderModel(p, "test-model")
		if err != nil {
			t.Errorf("InitProviderModel(%q) failed: %v", p, err)
		}
		if model == nil {
			t.Errorf("InitProviderModel(%q) returned nil model", p)
		}
	}

	// Test without env vars
	cleanAllLLMEnv(t)
	for _, p := range []string{"ollama", "litellm", "gemini", "openai", "anthropic", "openrouter", "deepseek", "groq"} {
		model, err := InitProviderModel(p, "test-model")
		if err != nil {
			t.Errorf("InitProviderModel(%q) without env failed: %v", p, err)
		}
		if model == nil {
			t.Errorf("InitProviderModel(%q) returned nil", p)
		}
	}

	// Custom without base URL
	t.Setenv("OPENAI_BASE_URL", "")
	if _, err := InitProviderModel("custom", "model"); err == nil {
		t.Errorf("expected error for custom provider without OPENAI_BASE_URL")
	}
}
