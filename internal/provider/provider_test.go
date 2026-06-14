package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestProvider_InvocationStruct(t *testing.T) {
	t.Parallel()
	inv := provider.Invocation{
		ToolName:  "Bash",
		ToolInput: []byte(`{"command":"ls"}`),
		CWD:       "/workspace",
		Raw:       []byte(`{}`),
	}
	if inv.ToolName != "Bash" {
		t.Errorf("ToolName: got %q", inv.ToolName)
	}
	if string(inv.ToolInput) != `{"command":"ls"}` {
		t.Errorf("ToolInput: %s", inv.ToolInput)
	}
	if inv.CWD != "/workspace" {
		t.Errorf("CWD: got %q", inv.CWD)
	}
	if string(inv.Raw) != `{}` {
		t.Errorf("Raw: %s", inv.Raw)
	}
}

// implementations is the canonical list of Provider implementations
// whose ReadInvocations/WriteDecision are fully functional.
//
// When a new Provider implementation lands, append it here so the
// conformance checks below cover it automatically.
func implementations() []provider.Provider {
	return []provider.Provider{
		provider.ClaudeProvider{},
		provider.CodexProvider{},
		provider.CursorProvider{},
		provider.CopilotProvider{},
		provider.AntigravityProvider{},
	}
}

func TestProvider_NamesAreLowercaseAndNonEmpty(t *testing.T) {
	t.Parallel()
	for _, p := range implementations() {
		name := p.Name()
		if name == "" {
			t.Errorf("provider %T: Name() is empty", p)
			continue
		}
		if name != strings.ToLower(name) {
			t.Errorf("provider %T: Name() %q is not lowercase", p, name)
		}
	}
}

func TestProvider_WriteDecision_AllValues(t *testing.T) {
	t.Parallel()
	for _, p := range implementations() {
		for _, dec := range []hook.Decision{hook.DecisionAllow, hook.DecisionDeny, hook.DecisionAsk} {
			t.Run(p.Name()+"/"+string(dec), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := p.WriteDecision(&buf, dec, "test"); err != nil {
					t.Fatalf("WriteDecision: %v", err)
				}
				if !json.Valid(buf.Bytes()) {
					t.Errorf("output is not valid JSON: %s", buf.String())
				}
			})
		}
	}
}
