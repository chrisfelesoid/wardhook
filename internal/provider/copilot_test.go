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

func TestCopilotProvider_ReadInvocations_EditFiles_Single(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "editFiles",
		"tool_input": {"files": ["src/a.ts"]}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(invs))
	}
	if invs[0].ToolName != "Edit" {
		t.Errorf("ToolName: got %q, want Edit", invs[0].ToolName)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "src/a.ts" {
		t.Errorf("file_path: got %v, want src/a.ts", ti["file_path"])
	}
}

func TestCopilotProvider_ReadInvocations_EditFiles_Multi(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "editFiles",
		"tool_input": {"files": ["src/a.ts", "src/b.ts", "src/c.ts"]}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 3 {
		t.Fatalf("expected 3 invocations, got %d", len(invs))
	}
	wantPaths := []string{"src/a.ts", "src/b.ts", "src/c.ts"}
	for i, inv := range invs {
		if inv.ToolName != "Edit" {
			t.Errorf("invs[%d].ToolName: got %q, want Edit", i, inv.ToolName)
		}
		if inv.CWD != "/workspace" {
			t.Errorf("invs[%d].CWD: %q", i, inv.CWD)
		}
		var ti map[string]any
		_ = json.Unmarshal(inv.ToolInput, &ti)
		if ti["file_path"] != wantPaths[i] {
			t.Errorf("invs[%d].file_path: got %v, want %q", i, ti["file_path"], wantPaths[i])
		}
	}
}

func TestCopilotProvider_ReadInvocations_EditFiles_Empty(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "editFiles",
		"tool_input": {"files": []}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1 invocation (empty editFiles), got %d", len(invs))
	}
	if invs[0].ToolName != "Edit" {
		t.Errorf("ToolName: got %q, want Edit", invs[0].ToolName)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if _, ok := ti["file_path"]; ok {
		t.Errorf("file_path should be absent for empty files, got %v", ti)
	}
}

func TestCopilotProvider_ReadInvocations_PreservesRawAcrossExpansion(t *testing.T) {
	t.Parallel()
	raw := `{
		"cwd": "/workspace",
		"tool_name": "editFiles",
		"tool_input": {"files": ["a", "b"]}
	}`
	p := provider.CopilotProvider{}
	invs, err := p.ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 2 {
		t.Fatalf("expected 2 invocations, got %d", len(invs))
	}
	if string(invs[0].Raw) != string(invs[1].Raw) {
		t.Errorf("Raw should be shared across expanded invocations")
	}
	if len(invs[0].Raw) == 0 {
		t.Errorf("Raw should not be empty")
	}
}
