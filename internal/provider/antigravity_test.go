package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestAntigravityProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.AntigravityProvider{}).Name() != "antigravity" {
		t.Errorf("Name: %q", (provider.AntigravityProvider{}).Name())
	}
}

// antigravityDecisionOut mirrors the wire shape so tests can assert on
// the parsed JSON without depending on map key ordering.
type antigravityDecisionOut struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func TestAntigravityProvider_WriteDecision_Allow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (provider.AntigravityProvider{}).WriteDecision(&buf, hook.DecisionAllow, ""); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var out antigravityDecisionOut
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out.Decision != "allow" {
		t.Errorf("decision: %q, want allow", out.Decision)
	}
	if out.Reason != "" {
		t.Errorf("reason: %q, want empty", out.Reason)
	}
	// reason should be omitted entirely when empty (omitempty)
	if strings.Contains(buf.String(), `"reason"`) {
		t.Errorf("empty reason should be omitted, got %s", buf.String())
	}
}

func TestAntigravityProvider_WriteDecision_Deny(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (provider.AntigravityProvider{}).WriteDecision(&buf, hook.DecisionDeny, "rule X matched"); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var out antigravityDecisionOut
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out.Decision != "deny" {
		t.Errorf("decision: %q, want deny", out.Decision)
	}
	if out.Reason != "rule X matched" {
		t.Errorf("reason: %q", out.Reason)
	}
}

func TestAntigravityProvider_WriteDecision_Ask(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (provider.AntigravityProvider{}).WriteDecision(&buf, hook.DecisionAsk, "parse error"); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var out antigravityDecisionOut
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out.Decision != "ask" {
		t.Errorf("decision: %q, want ask", out.Decision)
	}
}

func TestAntigravityProvider_WriteDecision_NoTopLevelHookSpecific(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (provider.AntigravityProvider{}).WriteDecision(&buf, hook.DecisionAllow, ""); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if _, present := raw["hookSpecificOutput"]; present {
		t.Errorf("output must not contain hookSpecificOutput (Claude-format): %s", buf.String())
	}
	if _, present := raw["decision"]; !present {
		t.Errorf("output must contain top-level decision: %s", buf.String())
	}
}
