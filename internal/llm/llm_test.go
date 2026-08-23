package llm

import (
	"os"
	"testing"
)

func TestGetProviderStatuses(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "test_key")
	defer os.Unsetenv("GEMINI_API_KEY")

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
		}
		if s.ID == "ollama" {
			foundOllama = true
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

func TestResolveProviderPriority(t *testing.T) {
	// Clean env
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("OLLAMA_URL")
	os.Unsetenv("LITELLM_BASE_URL")
	os.Unsetenv("LITELLM_URL")
	os.Unsetenv("LITELLM_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	// 1. LiteLLM active, Gemini active, OpenAI active -> LiteLLM wins over Gemini/OpenAI
	os.Setenv("LITELLM_BASE_URL", "https://litellm.example.com")
	os.Setenv("GEMINI_API_KEY", "gemini_key")
	os.Setenv("OPENAI_API_KEY", "openai_key")

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
	os.Setenv("OLLAMA_HOST", "http://localhost:11434")
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

	// Clean up
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("LITELLM_BASE_URL")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
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
}
