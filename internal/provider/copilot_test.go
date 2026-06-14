package provider_test

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestCopilotProvider_ReadInvocations_TerminalCommand(t *testing.T) {
	t.Parallel()
	raw := `{
		"timestamp": "2026-06-14T00:00:00Z",
		"cwd": "/workspace",
		"session_id": "s",
		"hook_event_name": "PreToolUse",
		"transcript_path": null,
		"tool_name": "runTerminalCommand",
		"tool_input": {"command": "rm -fr ./important"},
		"tool_use_id": "tu-1"
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(invs))
	}
	inv := invs[0]
	if inv.ToolName != "Bash" {
		t.Errorf("ToolName: got %q, want Bash", inv.ToolName)
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

func TestCopilotProvider_ReadInvocations_CreateFile_FilePathCamel(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "createFile",
		"tool_input": {"filePath": "src/new.ts"}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(invs))
	}
	if invs[0].ToolName != "Write" {
		t.Errorf("ToolName: got %q, want Write", invs[0].ToolName)
	}
	var ti map[string]any
	if uErr := json.Unmarshal(invs[0].ToolInput, &ti); uErr != nil {
		t.Fatalf("ToolInput unmarshal: %v", uErr)
	}
	if ti["file_path"] != "src/new.ts" {
		t.Errorf("file_path: got %v, want src/new.ts", ti["file_path"])
	}
}

func TestCopilotProvider_ReadInvocations_CreateFile_FilePathSnake(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "createFile",
		"tool_input": {"file_path": "src/new.ts"}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "src/new.ts" {
		t.Errorf("file_path: got %v, want src/new.ts", ti["file_path"])
	}
}
