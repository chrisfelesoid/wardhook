package hook_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

func TestReadInput_ValidJSON(t *testing.T) {
	t.Parallel()
	raw := `{
		"session_id": "s1",
		"cwd": "/workspace",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"}
	}`
	in, err := hook.ReadInput(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInput returned error: %v", err)
	}
	if in.SessionID != "s1" {
		t.Errorf("SessionID: got %q, want %q", in.SessionID, "s1")
	}
	if in.CWD != "/workspace" {
		t.Errorf("CWD: got %q, want %q", in.CWD, "/workspace")
	}
	if in.ToolName != "Bash" {
		t.Errorf("ToolName: got %q, want %q", in.ToolName, "Bash")
	}
	var ti map[string]any
	if uerr := json.Unmarshal(in.ToolInput, &ti); uerr != nil {
		t.Fatalf("ToolInput unmarshal: %v", uerr)
	}
	if ti["command"] != "ls" {
		t.Errorf("ToolInput.command: got %v, want ls", ti["command"])
	}
}

func TestReadInput_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := hook.ReadInput(strings.NewReader("{not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestWriteOutput_RoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := hook.WriteOutput(&buf, hook.DecisionDeny, "blocked by rule X"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	var out hook.Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
	}
	if out.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName: got %q, want PreToolUse", out.HookSpecificOutput.HookEventName)
	}
	if out.HookSpecificOutput.PermissionDecision != hook.DecisionDeny {
		t.Errorf("PermissionDecision: got %q, want deny", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.PermissionDecisionReason != "blocked by rule X" {
		t.Errorf("Reason: got %q", out.HookSpecificOutput.PermissionDecisionReason)
	}
}
