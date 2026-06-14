package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/provider"
)

const cursorSampleInput = `{
	"conversation_id": "conv-1",
	"generation_id": "gen-1",
	"model": "cursor-default",
	"hook_event_name": "preToolUse",
	"cursor_version": "0.45.0",
	"workspace_roots": ["/workspace"],
	"user_email": "tester@example.com",
	"transcript_path": null,
	"tool_name": "Shell",
	"tool_input": {"command": "rm -fr ./important"},
	"tool_use_id": "tu-1",
	"cwd": "/workspace",
	"agent_message": "delete legacy dir"
}`

func TestCursorProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.CursorProvider{}).Name() != "cursor" {
		t.Errorf("Name: %q", (provider.CursorProvider{}).Name())
	}
}

func TestCursorProvider_ReadInvocations_PreservesFields(t *testing.T) {
	t.Parallel()
	p := provider.CursorProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(cursorSampleInput))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1, got %d", len(invs))
	}
	inv := invs[0]
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

func TestCursorProvider_ReadInvocations_NormalizesShellToBash(t *testing.T) {
	t.Parallel()
	p := provider.CursorProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(cursorSampleInput))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1, got %d", len(invs))
	}
	if invs[0].ToolName != "Bash" {
		t.Errorf("ToolName should be normalized to Bash, got %q", invs[0].ToolName)
	}
}

func TestCursorProvider_ReadInvocations_PreservesNonShellToolName(t *testing.T) {
	t.Parallel()
	cases := []string{"Read", "Write", "Grep", "Delete", "Task", "MCP:custom-tool"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := `{"cwd":"/w","tool_name":"` + name + `","tool_input":{}}`
			p := provider.CursorProvider{}
			invs, err := p.ReadInvocations(strings.NewReader(raw))
			if err != nil {
				t.Fatalf("ReadInvocations: %v", err)
			}
			if len(invs) != 1 || invs[0].ToolName != name {
				t.Errorf("ToolName: got %v, want %q", invs, name)
			}
		})
	}
}

func TestCursorProvider_ReadInvocations_InvalidJSON(t *testing.T) {
	t.Parallel()
	p := provider.CursorProvider{}
	_, err := p.ReadInvocations(strings.NewReader("{not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCursorProvider_ReadInvocations_IgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "Shell",
		"tool_input": {"command": "ls"},
		"future_cursor_field": "future value",
		"another_unknown": {"nested": 1}
	}`
	p := provider.CursorProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Bash" {
		t.Errorf("ToolName: %v", invs)
	}
}

func TestCursorProvider_WriteDecision_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := provider.CursorProvider{}
	if err := p.WriteDecision(&buf, hook.DecisionDeny, "blocked by rule X"); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	var out struct {
		Permission   string `json:"permission"`
		AgentMessage string `json:"agent_message"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output JSON: %v\n%s", err, buf.String())
	}
	if out.Permission != "deny" {
		t.Errorf("permission: %q", out.Permission)
	}
	if out.AgentMessage != "blocked by rule X" {
		t.Errorf("agent_message: %q", out.AgentMessage)
	}
}

func TestCursorProvider_WriteDecision_AllDecisions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dec  hook.Decision
		want string
	}{
		{hook.DecisionAllow, "allow"},
		{hook.DecisionDeny, "deny"},
		{hook.DecisionAsk, "ask"},
	}
	for _, c := range cases {
		t.Run(string(c.dec), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			p := provider.CursorProvider{}
			if err := p.WriteDecision(&buf, c.dec, "r"); err != nil {
				t.Fatalf("WriteDecision: %v", err)
			}
			var out struct {
				Permission string `json:"permission"`
			}
			if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, buf.String())
			}
			if out.Permission != c.want {
				t.Errorf("permission: got %q, want %q", out.Permission, c.want)
			}
		})
	}
}

func TestCursorProvider_WriteDecision_EmptyReason(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := provider.CursorProvider{}
	if err := p.WriteDecision(&buf, hook.DecisionAllow, ""); err != nil {
		t.Fatalf("WriteDecision: %v", err)
	}
	if strings.Contains(buf.String(), "agent_message") {
		t.Errorf("agent_message should be omitted when reason is empty, got %s", buf.String())
	}
}
