package provider_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestCopilotProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.CopilotProvider{}).Name() != "copilot" {
		t.Errorf("Name: %q", (provider.CopilotProvider{}).Name())
	}
}

func TestCopilotProvider_WriteDecision_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := provider.CopilotProvider{}
	if err := p.WriteDecision(&buf, hook.DecisionDeny, "blocked"); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var out hook.Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output JSON: %v\n%s", err, buf.String())
	}
	if out.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName: %q", out.HookSpecificOutput.HookEventName)
	}
	if out.HookSpecificOutput.PermissionDecision != hook.DecisionDeny {
		t.Errorf("PermissionDecision: %q", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.PermissionDecisionReason != "blocked" {
		t.Errorf("Reason: %q", out.HookSpecificOutput.PermissionDecisionReason)
	}
}

func TestCopilotProvider_WriteDecision_AllDecisions(t *testing.T) {
	t.Parallel()
	cases := []hook.Decision{hook.DecisionAllow, hook.DecisionDeny, hook.DecisionAsk}
	for _, c := range cases {
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			p := provider.CopilotProvider{}
			if err := p.WriteDecision(&buf, c, "r"); err != nil {
				t.Fatalf("WriteDecision: %v", err)
			}
			var out hook.Output
			if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, buf.String())
			}
			if out.HookSpecificOutput.PermissionDecision != c {
				t.Errorf("PermissionDecision: got %q, want %q", out.HookSpecificOutput.PermissionDecision, c)
			}
		})
	}
}
