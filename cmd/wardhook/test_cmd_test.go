package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func tCfg(rules ...rule.Rule) *rule.Config {
	return &rule.Config{Version: 1, Rules: rules}
}

func TestResolveTool_FlagOverridesEverything(t *testing.T) {
	t.Parallel()
	cfg := tCfg(rule.Rule{Name: "r1", Tool: defaultTestTool,
		Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionDeny})
	got, err := resolveTool(cfg, []string{"r1"}, toolRead)
	if err != nil {
		t.Fatal(err)
	}
	if got != toolRead {
		t.Errorf("got %q, want Read", got)
	}
}

func TestResolveTool_SingleConcreteRule(t *testing.T) {
	t.Parallel()
	cfg := tCfg(rule.Rule{Name: "r1", Tool: toolRead,
		Match: rule.MatchSpec{}, Action: hook.DecisionDeny})
	got, err := resolveTool(cfg, []string{"r1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != toolRead {
		t.Errorf("got %q, want Read", got)
	}
}

func TestResolveTool_WildcardRuleFallsBackToBash(t *testing.T) {
	t.Parallel()
	cfg := tCfg(rule.Rule{Name: "r1", Tool: "*",
		Match: rule.MatchSpec{}, Action: hook.DecisionDeny})
	got, err := resolveTool(cfg, []string{"r1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultTestTool {
		t.Errorf("got %q, want Bash", got)
	}
}

func TestResolveTool_MultipleRulesFallBackToBash(t *testing.T) {
	t.Parallel()
	cfg := tCfg(
		rule.Rule{Name: "a", Tool: defaultTestTool, Action: hook.DecisionDeny},
		rule.Rule{Name: "b", Tool: toolRead, Action: hook.DecisionDeny},
	)
	got, err := resolveTool(cfg, []string{"a", "b"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultTestTool {
		t.Errorf("got %q, want Bash", got)
	}
}

func TestResolveTool_NoRulesFallBackToBash(t *testing.T) {
	t.Parallel()
	cfg := tCfg(rule.Rule{Name: "r1", Tool: toolRead, Action: hook.DecisionDeny})
	got, err := resolveTool(cfg, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultTestTool {
		t.Errorf("got %q, want Bash", got)
	}
}

func TestResolveTool_GrepRejected(t *testing.T) {
	t.Parallel()
	cfg := tCfg()
	_, err := resolveTool(cfg, nil, "Grep")
	if err == nil || !strings.Contains(err.Error(), "Grep") {
		t.Errorf("expected Grep rejection error, got %v", err)
	}
}

func TestResolveTool_UnknownToolFlagRejected(t *testing.T) {
	t.Parallel()
	cfg := tCfg()
	_, err := resolveTool(cfg, nil, "Mystery")
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestBuildToolInput_Bash(t *testing.T) {
	t.Parallel()
	got, err := buildToolInput("Bash", "rm -rf /tmp/foo")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if jerr := json.Unmarshal(got, &m); jerr != nil {
		t.Fatalf("json: %v", jerr)
	}
	if m["command"] != "rm -rf /tmp/foo" {
		t.Errorf("command field: %+v", m)
	}
}

func TestBuildToolInput_FilePathTools(t *testing.T) {
	t.Parallel()
	for _, name := range []string{toolRead, toolWrite, toolEdit, toolNotebookEdit} {
		raw, err := buildToolInput(name, "/etc/passwd")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if m["file_path"] != "/etc/passwd" {
			t.Errorf("%s field: %+v", name, m)
		}
	}
}

func TestBuildToolInput_PatternQueryURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		toolGlob:      "pattern",
		toolWebFetch:  "url",
		toolWebSearch: "query",
	}
	for tool, key := range cases {
		raw, err := buildToolInput(tool, "value-for-"+tool)
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if m[key] != "value-for-"+tool {
			t.Errorf("%s field %q: %+v", tool, key, m)
		}
	}
}

func TestBuildToolInput_RejectsUnsupported(t *testing.T) {
	t.Parallel()
	if _, err := buildToolInput("Grep", "foo"); err == nil {
		t.Error("expected error for Grep")
	}
	if _, err := buildToolInput("Mystery", "foo"); err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestFormatTrace_SingleRuleDeny(t *testing.T) {
	t.Parallel()
	tr := rule.Trace{
		Final:  hook.DecisionDeny,
		Reason: `[wardhook] denied by rule "block-rm-recursive": rm -fr ./important`,
		Commands: []rule.CommandTrace{{
			Command: parser.Command{
				Name:  "rm",
				Flags: map[string]struct{}{"f": {}, "r": {}},
				Args:  []string{"./important"},
				Raw:   "rm -fr ./important",
			},
			Rules: []rule.TraceEntry{{
				RuleName: "block-rm-recursive",
				Tool:     "Bash",
				Action:   hook.DecisionDeny,
				Outcome:  rule.OutcomeMatched,
			}},
		}},
	}
	header := headerInfo{
		ConfigPath:        "./wardhook.yaml",
		Tool:              "Bash",
		SelectedRuleNames: []string{"block-rm-recursive"},
		TotalRules:        5,
		InputCommand:      "rm -fr ./important",
	}
	var buf bytes.Buffer
	formatTrace(&buf, header, tr)
	out := buf.String()
	mustContain := []string{
		"config: ./wardhook.yaml",
		"tool:   Bash",
		"rules:  block-rm-recursive (1 of 5)",
		"input:  rm -fr ./important",
		"parsed commands (1):",
		"name=rm",
		`raw="rm -fr ./important"`,
		"block-rm-recursive (tool=Bash, action=deny)",
		"[0] MATCH -> deny",
		"final: deny",
		`reason: [wardhook] denied by rule "block-rm-recursive"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatTrace_NoSelectedRulesPrintsAll(t *testing.T) {
	t.Parallel()
	tr := rule.Trace{Final: hook.DecisionAllow}
	var buf bytes.Buffer
	formatTrace(&buf, headerInfo{
		ConfigPath:   "./wardhook.yaml",
		Tool:         "Bash",
		TotalRules:   3,
		InputCommand: "ls",
	}, tr)
	if !strings.Contains(buf.String(), "rules:  (all)") {
		t.Errorf("expected '(all)' marker, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "final: allow") {
		t.Errorf("expected final allow, got:\n%s", buf.String())
	}
}

func TestFormatTrace_InspectionFailedAnnotatesFinal(t *testing.T) {
	t.Parallel()
	tr := rule.Trace{
		Final:  hook.DecisionAsk,
		Reason: `[wardhook] asked: inspection failed for "bash -c \"broken"`,
		Commands: []rule.CommandTrace{{
			Command:          parser.Command{Name: "bash", Raw: `bash -c "broken`, InspectionFailed: true},
			InspectionFailed: true,
		}},
	}
	var buf bytes.Buffer
	formatTrace(&buf, headerInfo{
		ConfigPath:   "./wardhook.yaml",
		Tool:         "Bash",
		TotalRules:   0,
		InputCommand: `bash -c "broken`,
	}, tr)
	if !strings.Contains(buf.String(), "inspection_failed=true") {
		t.Errorf("expected inspection_failed marker, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "final: ask (forced by inspection_failed)") {
		t.Errorf("expected forced-ask annotation, got:\n%s", buf.String())
	}
}

func TestFormatTrace_ExceptShowsDetail(t *testing.T) {
	t.Parallel()
	tr := rule.Trace{
		Final: hook.DecisionAllow,
		Commands: []rule.CommandTrace{{
			Command: parser.Command{Name: "rm", Raw: "rm -rf /tmp/foo"},
			Rules: []rule.TraceEntry{{
				RuleName:     "block-rm",
				Tool:         "Bash",
				Action:       hook.DecisionDeny,
				Outcome:      rule.OutcomeExcepted,
				ExceptDetail: `glob "/tmp/**"`,
			}},
		}},
	}
	var buf bytes.Buffer
	formatTrace(&buf, headerInfo{
		ConfigPath: "./wardhook.yaml", Tool: "Bash", TotalRules: 1,
		InputCommand: "rm -rf /tmp/foo",
	}, tr)
	if !strings.Contains(buf.String(),
		`[0] MATCH -> EXCEPT (glob "/tmp/**") -> skip`) {
		t.Errorf("expected EXCEPT line, got:\n%s", buf.String())
	}
}

func runTestCmd(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(strings.NewReader(""), &stdout, &stderr, args)
	return code, stdout.String(), stderr.String()
}

func TestRunTest_SingleRuleMatch(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: block-rm",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    action: deny",
	})
	code, out, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "--rule", "block-rm", "rm -rf ./foo",
	})
	if code != 0 {
		t.Fatalf("exit %d (stderr=%s)", code, errStr)
	}
	if !strings.Contains(out, "final: deny") {
		t.Errorf("expected final: deny in:\n%s", out)
	}
	if !strings.Contains(out, "block-rm") {
		t.Errorf("expected rule name in:\n%s", out)
	}
}

func TestRunTest_NoRuleFlagPrintsAll(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: block-rm",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    action: deny",
	})
	code, out, _ := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "rm -rf ./foo",
	})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "rules:  (all)") {
		t.Errorf("expected (all) header in:\n%s", out)
	}
	if !strings.Contains(out, "final: deny") {
		t.Errorf("expected final: deny in:\n%s", out)
	}
}

func TestRunTest_UnknownRuleNameErrors(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: block-rm",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: deny",
	})
	code, _, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "--rule", "nope", "rm /x",
	})
	if code != exitTestArgError {
		t.Errorf("exit %d, want %d", code, exitTestArgError)
	}
	if !strings.Contains(errStr, "unknown rule") || !strings.Contains(errStr, "nope") {
		t.Errorf("stderr: %q", errStr)
	}
}

func TestRunTest_MissingConfigErrors(t *testing.T) {
	t.Parallel()
	code, _, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", "/no/such.yaml", "rm /x",
	})
	if code != exitTestArgError {
		t.Errorf("exit %d, want %d", code, exitTestArgError)
	}
	if !strings.Contains(errStr, "config") {
		t.Errorf("stderr: %q", errStr)
	}
}

func TestRunTest_WildcardRuleAgainstRead(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: deny-env",
		`    tool: "*"`,
		"    match:",
		"      glob:",
		"        mode: any",
		`        patterns: ["**/.env"]`,
		"    action: deny",
	})
	code, out, _ := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "--tool", "Read", "app/.env",
	})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "MATCH -> deny") {
		t.Errorf("expected MATCH -> deny in:\n%s", out)
	}
	if !strings.Contains(out, "final: deny") {
		t.Errorf("expected final: deny in:\n%s", out)
	}
}

func TestRunTest_GrepRejected(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules: []",
	})
	code, _, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "--tool", "Grep", "foo",
	})
	if code != exitTestArgError {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(errStr, "Grep") {
		t.Errorf("stderr: %q", errStr)
	}
}

func TestRunTest_RecursiveBashCExpansion(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"defaults:",
		"  cli_specs:",
		"    bash:",
		"      recurse:",
		"        flags: [c]",
		"rules:",
		"  - name: block-rm",
		"    tool: Bash",
		"    match: {command: rm, flags_all: [r, f]}",
		"    action: deny",
	})
	code, out, _ := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, `bash -c "rm -rf /etc/foo"`,
	})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "parsed commands (2)") {
		t.Errorf("expected 2 parsed commands in:\n%s", out)
	}
	if !strings.Contains(out, "final: deny") {
		t.Errorf("expected final: deny in:\n%s", out)
	}
}

func TestRunTest_MissingPositionalErrors(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules: []",
	})
	code, _, errStr := runTestCmd(t, []string{"wardhook", "test", "--config", cfg})
	if code != exitTestArgError {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(errStr, "usage") {
		t.Errorf("stderr: %q", errStr)
	}
}

func TestRunTest_BashParseErrorReturnsExit3(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: r",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: deny",
	})
	code, _, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg, "echo 'unclosed",
	})
	if code != exitTestParseError {
		t.Errorf("exit %d, want %d", code, exitTestParseError)
	}
	if !strings.Contains(errStr, "parse") {
		t.Errorf("stderr: %q", errStr)
	}
}

//nolint:paralleltest // t.Chdir forbids t.Parallel
func TestRunTest_DiscoversWardhookYaml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: block-rm",
		"    tool: Bash",
		"    match:",
		"      command: rm",
		"      flags_all: [r, f]",
		"    action: deny",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "wardhook.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	code, out, _ := runTestCmd(t, []string{"wardhook", "test", "rm -fr ./x"})
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out, "config: wardhook.yaml") {
		t.Errorf("stdout missing resolved config line: %q", out)
	}
	if !strings.Contains(out, "final: deny") {
		t.Errorf("stdout missing final deny: %q", out)
	}
}

//nolint:paralleltest // t.Chdir forbids t.Parallel
func TestRunTest_NoConfigInStandardLocationsErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, errStr := runTestCmd(t, []string{"wardhook", "test", "rm -fr ./x"})
	if code != exitTestArgError {
		t.Errorf("exit %d, want %d", code, exitTestArgError)
	}
	if !strings.Contains(errStr, "no config found in standard locations") {
		t.Errorf("stderr missing search-miss message: %q", errStr)
	}
}

func TestRunTest_TraceShowsNestedSubcommandPathInExcept(t *testing.T) {
	t.Parallel()
	cfg := writeConfig(t, []string{
		"version: 1",
		"rules:",
		"  - name: deny-gh-except-pr-create",
		"    tool: Bash",
		"    match:",
		"      command: gh",
		"    except:",
		"      subcommands_any:",
		"        - [pr, create]",
		"    action: deny",
	})
	code, out, errStr := runTestCmd(t, []string{
		"wardhook", "test", "--config", cfg,
		"--rule", "deny-gh-except-pr-create", "gh pr create --title hi",
	})
	if code != 0 {
		t.Fatalf("exit %d (stderr=%s)", code, errStr)
	}
	if !strings.Contains(out, "EXCEPT (subcommands_any [[pr create]])") {
		t.Errorf("trace should render nested subcommand path; got:\n%s", out)
	}
	if !strings.Contains(out, "final: allow") {
		t.Errorf("rule was excepted; final should be allow:\n%s", out)
	}
}
