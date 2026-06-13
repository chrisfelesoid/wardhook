package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type hookOut struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// writeConfig writes a YAML body to a temp file and returns its path.
// The body is given as lines that are joined with "\n" so that each
// Go source line in the caller stays tab-indented (editorconfig requires
// tabs in .go files) while the YAML content keeps the space indentation
// YAML 1.2 requires.
func writeConfig(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "wardhook.yaml")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return cfg
}

func runOnce(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := run(strings.NewReader(stdin), &out, &errBuf, args)
	return code, out.String(), errBuf.String()
}

func TestRun_AllowsByDefault_WhenNoConfig(t *testing.T) {
	t.Parallel()
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"ls"}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", "/no/such/file.yaml"}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if o.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
}

func TestRun_DeniesRmRf(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: block-rm-recursive",
		"    tool: Bash",
		"    match:",
		"      command: rm",
		"      flags_all: [r, f]",
		"    action: deny",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"rm -fr ./foo"}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("decision: %q (out=%s)", o.HookSpecificOutput.PermissionDecision, out)
	}
	if !strings.Contains(o.HookSpecificOutput.PermissionDecisionReason, "block-rm-recursive") {
		t.Errorf("reason should mention rule: %q", o.HookSpecificOutput.PermissionDecisionReason)
	}
}

func TestRun_ExceptExemptsTmp(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: block-rm-rf",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    except:",
		"      glob:",
		"        mode: all",
		`        patterns: ["/tmp/**"]`,
		"    action: deny",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"rm -fr /tmp/foo"}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
}

func TestRun_CrossToolEnvDenial(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: deny-sensitive-files",
		`    tool: "*"`,
		"    match:",
		"      glob:",
		"        mode: any",
		`        patterns: ["**/.env"]`,
		"    action: deny",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Read","tool_input":{"file_path":"./app/.env"}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("decision: %q (out=%s)", o.HookSpecificOutput.PermissionDecision, out)
	}
}

func TestRun_ConfigSyntaxError_AsksAndLogs(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{"not: valid: yaml: ::: invalid"})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"ls"}}`
	code, out, errStr := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "ask" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
	if !strings.Contains(errStr, "config") && !strings.Contains(errStr, "yaml") {
		t.Errorf("stderr should mention config error: %q", errStr)
	}
}

func TestRun_BashParseError_AsksAndLogs(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: r",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: deny",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"echo 'unclosed"}}`
	code, out, errStr := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "ask" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
	if !strings.Contains(errStr, "parse") {
		t.Errorf("stderr should mention parse error: %q", errStr)
	}
}

func TestRun_Validate_OK(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: r",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    action: deny",
	})
	code, out, errStr := runOnce(t, []string{"wardhook", "validate", "--config", cfg}, "")
	if code != 0 {
		t.Errorf("exit code: %d, stderr: %s", code, errStr)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("stdout should report OK: %q", out)
	}
}

func TestRun_Validate_BadYAML(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{"this: is: not yaml: :::"})
	code, _, errStr := runOnce(t, []string{"wardhook", "validate", "--config", cfg}, "")
	if code == 0 {
		t.Errorf("exit code should be non-zero for bad YAML")
	}
	if !strings.Contains(errStr, "error") && !strings.Contains(errStr, "yaml") {
		t.Errorf("stderr should mention the error: %q", errStr)
	}
}

func TestRun_Validate_MissingFile(t *testing.T) {
	t.Parallel()
	code, _, errStr := runOnce(t, []string{"wardhook", "validate", "--config", "/no/such.yaml"}, "")
	if code == 0 {
		t.Errorf("exit code should be non-zero for missing file")
	}
	if !strings.Contains(errStr, "no such") && !strings.Contains(errStr, "not exist") {
		t.Errorf("stderr should mention the missing file: %q", errStr)
	}
}

func TestRun_DispatchClaudeExplicit(t *testing.T) {
	t.Parallel()
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash","tool_input":{"command":"ls"}}`
	code, out, _ := runOnce(t, []string{"wardhook", "claude", "--config", "/no/such.yaml"}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if o.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
}

func TestRun_DispatchCodex_AllowsByDefaultWithNoConfig(t *testing.T) {
	t.Parallel()
	stdin := `{
		"session_id":"s","turn_id":"t","transcript_path":null,
		"cwd":"/workspace","hook_event_name":"PreToolUse",
		"model":"gpt-test","permission_mode":"default",
		"tool_name":"Bash","tool_input":{"command":"ls"},
		"tool_use_id":"u"
	}`
	code, out, _ := runOnce(t, []string{"wardhook", "codex", "--config", "/no/such/file.yaml"}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if o.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName: %q", o.HookSpecificOutput.HookEventName)
	}
	if o.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("decision: %q", o.HookSpecificOutput.PermissionDecision)
	}
}

func TestRun_DispatchGemini_NotImplemented(t *testing.T) {
	t.Parallel()
	stdin := `{}`
	code, _, errStr := runOnce(t, []string{"wardhook", "gemini"}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	if !strings.Contains(errStr, "panic") {
		t.Errorf("stderr should mention panic: %q", errStr)
	}
}

func TestRun_RecursiveEval_BashDashCDeny(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"defaults:",
		"  cli_specs:",
		"    bash:",
		"      recurse:",
		"        flags: [c]",
		"rules:",
		"  - name: block-rm-rf",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    action: deny",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash",` +
		`"tool_input":{"command":"bash -c \"rm -rf /etc/foo\""}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("decision: %q (out=%s)", o.HookSpecificOutput.PermissionDecision, out)
	}
}

func TestRun_RecursiveEval_BrokenInnerAsks(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"defaults:",
		"  cli_specs:",
		"    bash:",
		"      recurse:",
		"        flags: [c]",
		"rules: []",
	})
	stdin := `{"session_id":"s","cwd":"/workspace","tool_name":"Bash",` +
		`"tool_input":{"command":"bash -c \"echo 'unclosed\""}}`
	code, out, _ := runOnce(t, []string{"wardhook", "--config", cfg}, stdin)
	if code != 0 {
		t.Fatalf("exit code: %d", code)
	}
	var o hookOut
	_ = json.Unmarshal([]byte(out), &o)
	if o.HookSpecificOutput.PermissionDecision != "ask" {
		t.Errorf("decision: %q (out=%s)", o.HookSpecificOutput.PermissionDecision, out)
	}
}
