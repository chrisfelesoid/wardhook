package provider_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestGeminiProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.GeminiProvider{}).Name() != "gemini" {
		t.Errorf("Name: %q", (provider.GeminiProvider{}).Name())
	}
}

func TestGeminiProvider_ReadInvocation_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	_, _ = (provider.GeminiProvider{}).ReadInvocation(strings.NewReader("{}"))
}

func TestGeminiProvider_WriteDecision_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	_ = (provider.GeminiProvider{}).WriteDecision(nil, hook.DecisionAllow, "")
}
