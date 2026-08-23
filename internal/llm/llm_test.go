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
	for _, s := range statuses {
		if s.ID == "gemini" {
			foundGemini = true
			if !s.Active {
				t.Errorf("Expected Gemini provider to be active")
			}
		}
	}
	if !foundGemini {
		t.Errorf("Gemini provider missing in GetProviderStatuses")
	}
}

func TestResolveProviderAuto(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	pID, model, err := ResolveProvider("auto", "")
	if err != nil {
		t.Fatalf("ResolveProvider auto returned error: %v", err)
	}

	if pID != "openai" {
		t.Errorf("Expected auto to resolve to 'openai', got %q", pID)
	}
	if model != "gpt-4o-mini" {
		t.Errorf("Expected default model 'gpt-4o-mini', got %q", model)
	}
}

func TestResolveProviderExplicit(t *testing.T) {
	pID, model, err := ResolveProvider("anthropic", "claude-3-5-sonnet-latest")
	if err != nil {
		t.Fatalf("ResolveProvider explicit returned error: %v", err)
	}

	if pID != "anthropic" {
		t.Errorf("Expected 'anthropic', got %q", pID)
	}
	if model != "claude-3-5-sonnet-latest" {
		t.Errorf("Expected 'claude-3-5-sonnet-latest', got %q", model)
	}
}
