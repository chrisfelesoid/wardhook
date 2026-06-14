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

func TestAntigravityProvider_ReadInvocations_RunCommand(t *testing.T) {
	t.Parallel()
	raw := `{
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "rm -fr ./important"}
		},
		"stepIdx": 5,
		"conversationId": "conv-1",
		"workspacePaths": ["/workspace"],
		"transcriptPath": "/tmp/t.log",
		"artifactDirectoryPath": "/tmp/artifacts"
	}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
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
		t.Errorf("CWD: got %q, want /workspace", inv.CWD)
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

func TestAntigravityProvider_ReadInvocations_RunCommand_EmptyCommandLine(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"run_command","args":{"CommandLine":""}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Bash" {
		t.Fatalf("invs: %v", invs)
	}
	// Empty CommandLine: pass through original args object so BashParser
	// sees no command and treats it as a safe no-op.
	if !strings.Contains(string(invs[0].ToolInput), `"CommandLine"`) {
		t.Errorf("empty CommandLine should pass through original args, got %s", invs[0].ToolInput)
	}
}

func TestAntigravityProvider_ReadInvocations_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader("{not json"))
	if err == nil {
		t.Errorf("expected error for invalid JSON")
	}
}

func TestAntigravityProvider_ReadInvocations_ViewFile_FilePathSnake(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"view_file","args":{"file_path":"src/main.ts"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Read" {
		t.Fatalf("invs: %v", invs)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "src/main.ts" {
		t.Errorf("file_path: %v", ti["file_path"])
	}
}

func TestAntigravityProvider_ReadInvocations_ViewFile_FilePathPascal(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"view_file","args":{"FilePath":"src/main.ts"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Read" {
		t.Fatalf("invs: %v", invs)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "src/main.ts" {
		t.Errorf("file_path: %v", ti["file_path"])
	}
}

func TestAntigravityProvider_ReadInvocations_ViewFile_Path(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"view_file","args":{"Path":"src/main.ts"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Read" {
		t.Fatalf("invs: %v", invs)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "src/main.ts" {
		t.Errorf("file_path: %v", ti["file_path"])
	}
}

func TestAntigravityProvider_ReadInvocations_EditFile_PriorityOrder(t *testing.T) {
	t.Parallel()
	// All three keys present: file_path (snake) wins, then FilePath, then Path.
	raw := `{"toolCall":{"name":"edit_file","args":{"file_path":"a","FilePath":"b","Path":"c"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Edit" {
		t.Fatalf("invs: %v", invs)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "a" {
		t.Errorf("priority order: got file_path=%v, want a (snake_case wins)", ti["file_path"])
	}
}

func TestAntigravityProvider_ReadInvocations_WriteFile_FilePathPascal(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"write_file","args":{"FilePath":"out.txt"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Write" {
		t.Fatalf("invs: %v", invs)
	}
	var ti map[string]any
	_ = json.Unmarshal(invs[0].ToolInput, &ti)
	if ti["file_path"] != "out.txt" {
		t.Errorf("file_path: %v", ti["file_path"])
	}
}

func TestAntigravityProvider_ReadInvocations_ListDir_PassThrough(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"list_dir","args":{"Path":"/w"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "list_dir" {
		t.Errorf("ToolName should pass through unchanged, got %v", invs)
	}
	// args should be unchanged
	if !strings.Contains(string(invs[0].ToolInput), `"Path"`) {
		t.Errorf("args should pass through unchanged, got %s", invs[0].ToolInput)
	}
}

func TestAntigravityProvider_ReadInvocations_MCPTool_PassThrough(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"myserver/foo","args":{"x":1}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "myserver/foo" {
		t.Errorf("MCP tool name should pass through, got %v", invs)
	}
}

func TestAntigravityProvider_ReadInvocations_UnknownTool_PassThrough(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"future_tool","args":{}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "future_tool" {
		t.Errorf("unknown tool name should pass through, got %v", invs)
	}
}

func TestAntigravityProvider_ReadInvocations_WorkspacePaths_First(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls"}},"workspacePaths":["/a","/b"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if invs[0].CWD != "/a" {
		t.Errorf("CWD: got %q, want /a (first workspace)", invs[0].CWD)
	}
}

func TestAntigravityProvider_ReadInvocations_WorkspacePaths_Empty(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls"}},"workspacePaths":[]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if invs[0].CWD != "" {
		t.Errorf("CWD: got %q, want empty", invs[0].CWD)
	}
}

func TestAntigravityProvider_ReadInvocations_WorkspacePaths_Missing(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls"}}}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if invs[0].CWD != "" {
		t.Errorf("CWD: got %q, want empty when workspacePaths missing", invs[0].CWD)
	}
}

func TestAntigravityProvider_ReadInvocations_IgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	raw := `{
		"toolCall": {"name":"run_command","args":{"CommandLine":"ls"}},
		"workspacePaths": ["/w"],
		"stepIdx": 42,
		"conversationId": "c-1",
		"transcriptPath": "/tmp/t.log",
		"artifactDirectoryPath": "/tmp/art",
		"futureField": "should-be-ignored"
	}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ToolName != "Bash" {
		t.Errorf("invs: %v", invs)
	}
}

func TestAntigravityProvider_ReadInvocations_PreservesRaw(t *testing.T) {
	t.Parallel()
	raw := `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls"}},"workspacePaths":["/w"]}`
	invs, err := (provider.AntigravityProvider{}).ReadInvocations(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if string(invs[0].Raw) != raw {
		t.Errorf("Raw should preserve original JSON verbatim:\n got: %s\nwant: %s", invs[0].Raw, raw)
	}
}
