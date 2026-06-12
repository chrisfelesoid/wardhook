package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestClaudeProvider_Name(t *testing.T) {
	t.Parallel()
	p := provider.ClaudeProvider{}
	if p.Name() != "claude" {
		t.Errorf("Name: got %q, want claude", p.Name())
	}
}

func TestClaudeProvider_ReadInvocation_PreservesFields(t *testing.T) {
	t.Parallel()
	raw := `{
		"session_id": "s",
		"cwd": "/workspace",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"}
	}`
	p := provider.ClaudeProvider{}
	inv, err := p.ReadInvocation(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocation: %v", err)
	}
	if inv.ToolName != "Bash" {
		t.Errorf("ToolName: %q", inv.ToolName)
	}
	if inv.CWD != "/workspace" {
		t.Errorf("CWD: %q", inv.CWD)
	}
	var ti map[string]any
	if uErr := json.Unmarshal(inv.ToolInput, &ti); uErr != nil {
		t.Fatalf("ToolInput unmarshal: %v", uErr)
	}
	if ti["command"] != "ls" {
		t.Errorf("ToolInput.command: %v", ti["command"])
	}
	if len(inv.Raw) == 0 {
		t.Errorf("Raw should not be empty")
	}
}

func TestClaudeProvider_ReadInvocation_InvalidJSON(t *testing.T) {
	t.Parallel()
	p := provider.ClaudeProvider{}
	_, err := p.ReadInvocation(strings.NewReader("{not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClaudeProvider_WriteDecision_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := provider.ClaudeProvider{}
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
