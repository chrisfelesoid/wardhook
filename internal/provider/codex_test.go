package provider_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestCodexProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.CodexProvider{}).Name() != "codex" {
		t.Errorf("Name: %q", (provider.CodexProvider{}).Name())
	}
}

func TestCodexProvider_ReadInvocation_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	_, _ = (provider.CodexProvider{}).ReadInvocation(strings.NewReader("{}"))
}

func TestCodexProvider_WriteDecision_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	_ = (provider.CodexProvider{}).WriteDecision(nil, hook.DecisionAllow, "")
}
