package rule_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "wardhook.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// yamlMinimal returns a baseline rules document used by several tests.
// We build YAML by string-joining so each Go source line stays tab-indented
// (editorconfig requires tabs in .go files) while the YAML content uses
// the space indentation YAML 1.2 requires.
func yamlMinimal() string {
	return strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: rm",
		"      flags_all: [r, f]",
		"    action: deny",
		"",
	}, "\n")
}

func TestLoad_Minimal(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, yamlMinimal())
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("version: %d", cfg.Version)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("rules: %d", len(cfg.Rules))
	}
	r := cfg.Rules[0]
	if r.Name != "r1" || r.Tool != "Bash" {
		t.Errorf("rule[0]: %+v", r)
	}
	if r.Action != hook.DecisionDeny {
		t.Errorf("action: %q", r.Action)
	}
	if r.Match.Command != "rm" {
		t.Errorf("match.command: %q", r.Match.Command)
	}
	if len(r.Match.FlagsAll) != 2 {
		t.Errorf("flags_all: %v", r.Match.FlagsAll)
	}
}

func TestLoad_FileNotExist(t *testing.T) {
	t.Parallel()
	_, err := rule.Load("/no/such/path.yaml")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestLoad_UnknownKey_Strict(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: deny",
		"    bogus_key: oops",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected strict YAML to reject unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_key") {
		t.Errorf("error should mention bogus_key: %v", err)
	}
}

func TestLoad_InvalidAction(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: block",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected validation error for unknown action, got nil")
	}
}

func TestLoad_MissingVersion(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match: {command: rm}",
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestLoad_WithExcept(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: rm",
		"      flags_all: [r, f]",
		"    except:",
		"      glob:",
		"        mode: all",
		`        patterns: ["/tmp/**"]`,
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Rules[0].Except == nil {
		t.Fatal("Except should not be nil")
	}
	if cfg.Rules[0].Except.Glob == nil {
		t.Fatal("Except.Glob should not be nil")
	}
	if cfg.Rules[0].Except.Glob.Mode != rule.GlobModeAll {
		t.Errorf("Except.Glob.Mode: %v", cfg.Rules[0].Except.Glob.Mode)
	}
}

func TestLoad_CLISpecs_UserDeclared(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"defaults:",
		"  cli_specs:",
		"    mycli:",
		"      value_taking_flags: [profile, p]",
		"      recurse:",
		"        terminator: true",
		"  recursive_max_depth: 5",
		"rules: []",
		"",
	}, "\n"))
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	mycli, ok := cfg.Defaults.CLISpecs["mycli"]
	if !ok {
		t.Fatal("mycli missing")
	}
	if len(mycli.ValueTakingFlags) != 2 {
		t.Errorf("mycli.value_taking_flags: %v", mycli.ValueTakingFlags)
	}
	if !mycli.Recurse.Terminator {
		t.Errorf("mycli.recurse.terminator should be true")
	}
	if cfg.Defaults.RecursiveMaxDepth != 5 {
		t.Errorf("max_depth: %d", cfg.Defaults.RecursiveMaxDepth)
	}
}

func TestLoad_NegativeMaxDepth_Rejected(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"defaults:",
		"  recursive_max_depth: -1",
		"rules: []",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil {
		t.Fatal("expected error for negative max depth")
	}
}

func TestLoad_DefaultsCLISpecs_UnsetUsesBuiltin(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules: []",
		"",
	}, "\n"))
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	resolved := cfg.Defaults.ResolvedCLISpecs()
	if _, ok := resolved["bash"]; !ok {
		t.Errorf("builtin should include bash: %v", resolved)
	}
	if _, ok := resolved["docker"]; !ok {
		t.Errorf("builtin should include docker: %v", resolved)
	}
	if cfg.Defaults.ResolvedRecursiveMaxDepth() != 3 {
		t.Errorf("default max depth not 3")
	}
}

func TestResolvedValueTakingFlags_PerCommand(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: tf",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["env/prod"]`,
		"        - name: var-file",
		"          glob:",
		"            mode: any",
		`            patterns: ["secret*"]`,
		"    action: deny",
		"  - name: kc",
		"    tool: Bash",
		"    match:",
		"      command: kubectl",
		"      flag_values:",
		"        - name: n",
		"          glob:",
		"            mode: any",
		`            patterns: ["prod"]`,
		"    action: deny",
		"",
	}, "\n"))
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.ResolvedValueTakingFlags()
	if _, ok := got["terraform"]["chdir"]; !ok {
		t.Errorf("terraform.chdir missing: %v", got)
	}
	if _, ok := got["terraform"]["var-file"]; !ok {
		t.Errorf("terraform.var-file missing: %v", got)
	}
	if _, ok := got["kubectl"]["n"]; !ok {
		t.Errorf("kubectl.n missing: %v", got)
	}
}

func TestResolvedValueTakingFlags_WildcardCommand(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: anywhere",
		"    tool: Bash",
		"    match:",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["x"]`,
		"    action: deny",
		"",
	}, "\n"))
	cfg, _ := rule.Load(p)
	got := cfg.ResolvedValueTakingFlags()
	if _, ok := got[""]["chdir"]; !ok {
		t.Errorf("wildcard '' key should hold chdir: %v", got)
	}
}

func TestResolvedValueTakingFlags_IncludesAltNamesViaFlagAliases(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: tf",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_aliases:",
		`        c: ["chdir"]`,
		"      flag_values:",
		"        - name: c",
		"          glob:",
		"            mode: any",
		`            patterns: ["env/prod"]`,
		"    action: deny",
		"",
	}, "\n"))
	cfg, _ := rule.Load(p)
	got := cfg.ResolvedValueTakingFlags()
	if _, ok := got["terraform"]["c"]; !ok {
		t.Errorf("canonical c missing: %v", got)
	}
	// Alt names must also enter the set so the parser can consume the
	// space-form value when the command line uses the alt spelling
	// (e.g. `-chdir foo`). Without this, `foo` would fall to positional
	// args and the rule's value glob would not match.
	if _, ok := got["terraform"]["chdir"]; !ok {
		t.Errorf("alt name chdir should also be in set: %v", got)
	}
}

func TestResolvedValueTakingFlags_DedupAcrossRules(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["a"]`,
		"    action: deny",
		"  - name: r2",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["b"]`,
		"    action: deny",
		"",
	}, "\n"))
	cfg, _ := rule.Load(p)
	got := cfg.ResolvedValueTakingFlags()
	if n := len(got["terraform"]); n != 1 {
		t.Errorf("dedup expected size 1, got %d: %v", n, got["terraform"])
	}
}

func TestLoad_FlagValues_EmptyName(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		`        - name: ""`,
		"          glob:",
		"            mode: any",
		`            patterns: ["x"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "non-empty") {
		t.Errorf("expected non-empty name error, got %v", err)
	}
}

func TestLoad_FlagValues_EmptyGlob(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		"            patterns: []",
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "at least one pattern") {
		t.Errorf("expected empty patterns error, got %v", err)
	}
}

func TestLoad_FlagValues_GlobMissingMode(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		`            patterns: ["env/prod"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "mode is required") {
		t.Errorf("expected mode-required error, got %v", err)
	}
}

func TestLoad_FlagValues_DuplicateName(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["a"]`,
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["b"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("expected duplicate name error, got %v", err)
	}
}

func TestLoad_FlagValues_DuplicateViaAlias(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_aliases:",
		`        c: ["chdir"]`,
		"      flag_values:",
		"        - name: c",
		"          glob:",
		"            mode: any",
		`            patterns: ["a"]`,
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["b"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("expected duplicate via alias error, got %v", err)
	}
}

func TestLoad_FlagValues_DashPrefixName(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: -chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["a"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), `must not start with "-"`) {
		t.Errorf("expected dash-prefix error, got %v", err)
	}
}

func TestLoad_FlagValues_InvalidGlob(t *testing.T) {
	t.Parallel()
	// doublestar.Match rejects "[" without close as ErrBadPattern.
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: terraform",
		"      flag_values:",
		"        - name: chdir",
		"          glob:",
		"            mode: any",
		`            patterns: ["["]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid glob error, got %v", err)
	}
}

func TestLoad_Glob_MissingMode(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      glob:",
		`        patterns: ["/etc/**"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "mode is required") {
		t.Errorf("expected mode-required error, got %v", err)
	}
}

func TestLoad_Glob_InvalidMode(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      glob:",
		"        mode: maybe",
		`        patterns: ["/etc/**"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), `must be "any" or "all"`) {
		t.Errorf("expected invalid mode error, got %v", err)
	}
}

func TestLoad_Glob_EmptyPatterns(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      glob:",
		"        mode: any",
		"        patterns: []",
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "at least one pattern") {
		t.Errorf("expected empty-patterns error, got %v", err)
	}
}

func TestLoad_Glob_InvalidPattern(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      glob:",
		"        mode: any",
		`        patterns: ["["]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid pattern error, got %v", err)
	}
}

func TestLoad_LegacyPathGlob_Rejected(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		`      path_glob: ["/etc/**"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "path_glob") {
		t.Errorf("expected unknown-field error for legacy path_glob, got %v", err)
	}
}

func TestLoad_LegacyRecursiveEval_Rejected(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"defaults:",
		"  recursive_eval:",
		"    bash: [c]",
		"rules: []",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "recursive_eval") {
		t.Errorf("expected unknown-field error for legacy recursive_eval, got %v", err)
	}
}

func TestLoad_CLISpecs_InvalidSpec(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"defaults:",
		"  cli_specs:",
		"    bad:",
		"      recurse:",
		"        subcommands:",
		"          run:",
		"            skip: -1",
		"rules: []",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "skip must be >= 0") {
		t.Errorf("expected negative-skip error, got %v", err)
	}
}

func TestLoad_Regex_MissingMode(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      regex:",
		`        patterns: ["^x$"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "mode is required") {
		t.Errorf("expected mode-required error, got %v", err)
	}
}

func TestLoad_Regex_InvalidMode(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      regex:",
		"        mode: maybe",
		`        patterns: ["^x$"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), `must be "any" or "all"`) {
		t.Errorf("expected invalid mode error, got %v", err)
	}
}

func TestLoad_Regex_EmptyPatterns(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      regex:",
		"        mode: any",
		"        patterns: []",
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "at least one pattern") {
		t.Errorf("expected empty patterns error, got %v", err)
	}
}

func TestLoad_Regex_InvalidPattern(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      regex:",
		"        mode: any",
		`        patterns: ["[invalid"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "invalid:") {
		t.Errorf("expected invalid regex error, got %v", err)
	}
}

func TestLoad_LegacyArgsRegex_Rejected(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		`      args_regex: ["^x$"]`,
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "args_regex") {
		t.Errorf("expected unknown-field error for legacy args_regex, got %v", err)
	}
}

func TestLoad_FlagValues_GlobOrRegexRequired(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: kubectl",
		"      flag_values:",
		"        - name: n",
		"    action: deny",
		"",
	}, "\n"))
	if _, err := rule.Load(p); err == nil ||
		!strings.Contains(err.Error(), "at least one of glob, regex") {
		t.Errorf("expected glob-or-regex required error, got %v", err)
	}
}

func TestLoad_FlagValues_Regex(t *testing.T) {
	t.Parallel()
	p := writeYAML(t, strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: r1",
		"    tool: Bash",
		"    match:",
		"      command: kubectl",
		"      flag_values:",
		"        - name: n",
		"          regex:",
		"            mode: any",
		`            patterns: ['^prod-\d+$']`,
		"    action: deny",
		"",
	}, "\n"))
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Rules[0].Match.FlagValues[0].Regex == nil {
		t.Error("FlagValues[0].Regex should be populated")
	}
}

func TestLoad_SubcommandsOnBash_OK(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: deny-git-push",
		"    tool: Bash",
		"    match:",
		"      command: git",
		"      subcommands_any: [push, fetch]",
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	cfg, err := rule.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("rules: %d", len(cfg.Rules))
	}
	got := cfg.Rules[0].Match.SubcommandsAny
	if len(got) != 2 || got[0] != "push" || got[1] != "fetch" {
		t.Errorf("subcommands_any: %v", got)
	}
}

func TestLoad_SubcommandsOnRead_Errors(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: bogus",
		"    tool: Read",
		"    match:",
		"      subcommands_any: [push]",
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected Load to fail for subcommands on tool: Read")
	}
	msg := err.Error()
	if !strings.Contains(msg, "subcommands") || !strings.Contains(msg, "Bash") {
		t.Errorf("error should explain subcommands+Bash constraint: %q", msg)
	}
	if !strings.Contains(msg, "bogus") {
		t.Errorf("error should mention rule name: %q", msg)
	}
}

func TestLoad_SubcommandsOnWildcard_Errors(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: bogus-wildcard",
		`    tool: "*"`,
		"    match:",
		"      subcommands_any: [push]",
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected Load to fail for subcommands on tool: *")
	}
	msg := err.Error()
	if !strings.Contains(msg, "subcommands") || !strings.Contains(msg, "Bash") {
		t.Errorf("error should explain subcommands+Bash constraint: %q", msg)
	}
	if !strings.Contains(msg, "bogus-wildcard") {
		t.Errorf("error should mention rule name: %q", msg)
	}
}

func TestLoad_SubcommandsInExceptOnNonBash_Errors(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: bogus-except",
		"    tool: Write",
		"    match:",
		`      glob: { mode: any, patterns: ["**/.env"] }`,
		"    except:",
		"      subcommands_any: [status]",
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	_, err := rule.Load(p)
	if err == nil {
		t.Fatal("expected Load to fail for subcommands in except on tool: Write")
	}
	msg := err.Error()
	if !strings.Contains(msg, "subcommands") || !strings.Contains(msg, "Bash") {
		t.Errorf("error should explain subcommands+Bash constraint: %q", msg)
	}
	if !strings.Contains(msg, "bogus-except") {
		t.Errorf("error should mention rule name: %q", msg)
	}
}

func TestLoad_SubcommandsEmptyArrayOnRead_OK(t *testing.T) {
	t.Parallel()
	// An explicitly empty array is treated as "not specified" — it
	// must not error even on non-Bash, matching the documented
	// "len > 0 triggers the tool check" rule.
	body := strings.Join([]string{
		"version: 1",
		"rules:",
		"  - name: empty-array-on-read",
		"    tool: Read",
		"    match:",
		"      subcommands_any: []",
		`      glob: { mode: any, patterns: ["**/.env"] }`,
		"    action: deny",
		"",
	}, "\n")
	p := writeYAML(t, body)
	if _, err := rule.Load(p); err != nil {
		t.Fatalf("Load should accept empty subcommands_any on non-Bash: %v", err)
	}
}
