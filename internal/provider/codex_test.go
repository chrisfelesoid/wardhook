package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

const codexSampleInput = `{
	"session_id": "sess-1",
	"turn_id": "turn-1",
	"transcript_path": null,
	"cwd": "/workspace",
	"hook_event_name": "PreToolUse",
	"model": "gpt-test",
	"permission_mode": "default",
	"tool_name": "Bash",
	"tool_input": {"command": "rm -fr ./important"},
	"tool_use_id": "tool-1"
}`

func TestCodexProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.CodexProvider{}).Name() != "codex" {
		t.Errorf("Name: %q", (provider.CodexProvider{}).Name())
	}
}

func TestCodexProvider_ReadInvocations_PreservesFields(t *testing.T) {
	t.Parallel()
	p := provider.CodexProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(codexSampleInput))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1, got %d", len(invs))
	}
	inv := invs[0]
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
	if ti["command"] != "rm -fr ./important" {
		t.Errorf("ToolInput.command: %v", ti["command"])
	}
	if len(inv.Raw) == 0 {
		t.Errorf("Raw should not be empty")
	}
}

func TestCodexProvider_ReadInvocations_InvalidJSON(t *testing.T) {
	t.Parallel()
	p := provider.CodexProvider{}
	_, err := p.ReadInvocations(strings.NewReader("{not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCodexProvider_ReadInvocations_IgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	raw := `{
		"session_id": "s",
		"cwd": "/workspace",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"},
		"future_codex_field": "future value",
		"another_unknown": {"nested": 1}
	}`
	p := provider.CodexProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1, got %d", len(invs))
	}
	if invs[0].ToolName != "Bash" {
		t.Errorf("ToolName: %q", invs[0].ToolName)
	}
}

func TestCodexProvider_WriteDecision_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := provider.CodexProvider{}
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
