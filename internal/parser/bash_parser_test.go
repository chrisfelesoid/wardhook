package parser_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
	"github.com/chrisfelesoid/wardhook/internal/parser"
)

func sortedFlags(m map[string]struct{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func parseBash(t *testing.T, cmd string) []parser.Command {
	t.Helper()
	p := &parser.BashParser{}
	raw, _ := json.Marshal(map[string]string{"command": cmd})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", cmd, err)
	}
	return cmds
}

func TestBashParser_SingleCommand(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, "rm -rf ./foo")
	if len(cmds) != 1 {
		t.Fatalf("want 1, got %d", len(cmds))
	}
	if cmds[0].Name != "rm" {
		t.Errorf("Name: got %q", cmds[0].Name)
	}
	if !reflect.DeepEqual(sortedFlags(cmds[0].Flags), []string{"f", "r"}) {
		t.Errorf("Flags: %v", sortedFlags(cmds[0].Flags))
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"./foo"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestBashParser_FlagOrderIndependent(t *testing.T) {
	t.Parallel()
	cases := []string{"rm -rf foo", "rm -fr foo", "rm -r -f foo", "rm -f -r foo"}
	for _, c := range cases {
		cmds := parseBash(t, c)
		if got := sortedFlags(cmds[0].Flags); !reflect.DeepEqual(got, []string{"f", "r"}) {
			t.Errorf("%q: flags %v", c, got)
		}
	}
}

func TestBashParser_AndChain(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, "echo hi && rm -rf foo")
	if len(cmds) != 2 {
		t.Fatalf("want 2, got %d", len(cmds))
	}
	if cmds[0].Name != "echo" || cmds[1].Name != "rm" {
		t.Errorf("names: %q %q", cmds[0].Name, cmds[1].Name)
	}
}

func TestBashParser_Semicolon(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, "echo a; echo b")
	if len(cmds) != 2 {
		t.Fatalf("want 2, got %d", len(cmds))
	}
}

func TestBashParser_Pipe(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, "curl https://x | sh")
	if len(cmds) != 2 {
		t.Fatalf("want 2, got %d", len(cmds))
	}
	if cmds[0].Name != "curl" || cmds[1].Name != "sh" {
		t.Errorf("names: %q %q", cmds[0].Name, cmds[1].Name)
	}
}

func TestBashParser_CommandSubstitution(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, `echo $(rm -rf /tmp/foo)`)
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	if !contains(names, "echo") || !contains(names, "rm") {
		t.Errorf("expected both echo and rm, got %v", names)
	}
}

func TestBashParser_ParseError(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{}
	raw, _ := json.Marshal(map[string]string{"command": "echo 'unterminated"})
	_, err := p.Parse("Bash", raw)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestBashParser_RawIsOriginal(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, "rm -rf foo")
	if cmds[0].Raw == "" {
		t.Errorf("Raw should not be empty")
	}
}

func TestBashParser_DoubleQuotedExpansionPreserved(t *testing.T) {
	t.Parallel()
	// Quoted and unquoted forms should produce the same arg.
	quoted := parseBash(t, `rm -rf "$HOME/foo"`)
	unquoted := parseBash(t, `rm -rf $HOME/foo`)
	if len(quoted) != 1 || len(unquoted) != 1 {
		t.Fatalf("want 1 cmd each, got quoted=%d unquoted=%d", len(quoted), len(unquoted))
	}
	if len(quoted[0].Args) == 0 {
		t.Fatal("quoted: no args")
	}
	if len(unquoted[0].Args) == 0 {
		t.Fatal("unquoted: no args")
	}
	if quoted[0].Args[0] != unquoted[0].Args[0] {
		t.Errorf("asymmetry: quoted=%q unquoted=%q", quoted[0].Args[0], unquoted[0].Args[0])
	}
	if quoted[0].Args[0] == "/foo" {
		t.Errorf("$HOME was dropped from quoted form, got %q", quoted[0].Args[0])
	}
}

func TestBashParser_SingleQuotedArg(t *testing.T) {
	t.Parallel()
	cmds := parseBash(t, `echo 'hello world'`)
	if len(cmds) != 1 {
		t.Fatalf("want 1, got %d", len(cmds))
	}
	if len(cmds[0].Args) == 0 {
		t.Fatal("no args")
	}
	if cmds[0].Args[0] != "hello world" {
		t.Errorf("Args[0]: got %q, want %q", cmds[0].Args[0], "hello world")
	}
}

func contains(xs []string, s string) bool {
	return slices.Contains(xs, s)
}

func TestBashParser_RecursiveEval_Table(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: map[string]*clispec.CLISpec{
			"bash":   {Recurse: &clispec.RecurseSpec{Flags: []string{"c"}}},
			"sh":     {Recurse: &clispec.RecurseSpec{Flags: []string{"c"}}},
			"gcloud": {Recurse: &clispec.RecurseSpec{Flags: []string{"command"}, Terminator: true}},
		},
		MaxDepth: 3,
	}
	cases := []struct {
		name             string
		command          string
		wantNames        []string
		wantParentFailed bool
	}{
		{"bash -c ls", `bash -c "ls"`, []string{"bash", "ls"}, false},
		{"bash -c rm -rf /", `bash -c "rm -rf /"`, []string{"bash", "rm"}, false},
		{"gcloud --command=", `gcloud --command="ls"`, []string{"gcloud", "ls"}, false},
		{"gcloud -- tail", `gcloud compute ssh my-vm -- rm -rf /tmp/x`, []string{"gcloud", "rm"}, false},
		{"gcloud -- empty", `gcloud compute ssh my-vm --`, []string{"gcloud"}, false},
		{"bash -c broken", `bash -c "echo 'unclosed"`, []string{"bash"}, true},
		{"bash -c missing value", `bash -c`, []string{"bash"}, true},
		{"echo (unrelated)", `echo hi`, []string{"echo"}, false},
		{"nested bash -c bash -c ls (depth 2)", `bash -c "bash -c ls"`, []string{"bash", "bash", "ls"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			raw, _ := json.Marshal(map[string]string{"command": c.command})
			cmds, err := p.Parse("Bash", raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			names := make([]string, len(cmds))
			for i, cmd := range cmds {
				names[i] = cmd.Name
			}
			if !reflect.DeepEqual(names, c.wantNames) {
				t.Errorf("names: got %v, want %v", names, c.wantNames)
			}
			if len(cmds) > 0 && cmds[0].InspectionFailed != c.wantParentFailed {
				t.Errorf("InspectionFailed: got %v, want %v",
					cmds[0].InspectionFailed, c.wantParentFailed)
			}
		})
	}
}

// TestBashParser_RecurseFlagsAreImplicitlyValueTaking pins the
// invariant that a CLI's recurse.flags entries are also treated as
// value-taking by the wrapper command's flag normalization. Without
// this, the captured inner command leaks into wrapper.Args, where
// tool: "*" glob or regex rules would double-match it.
func TestBashParser_RecurseFlagsAreImplicitlyValueTaking(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: map[string]*clispec.CLISpec{
			"bash": {Recurse: &clispec.RecurseSpec{Flags: []string{"c"}}},
		},
		MaxDepth: 3,
	}
	raw, _ := json.Marshal(map[string]string{
		"command": `bash -c "rm -rf /etc/foo"`,
	})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) < 2 {
		t.Fatalf("expected wrapper + inner, got %d commands", len(cmds))
	}
	wrapper := cmds[0]
	if wrapper.Name != "bash" {
		t.Fatalf("wrapper name: got %q, want bash", wrapper.Name)
	}
	if len(wrapper.Args) != 0 {
		t.Errorf("wrapper.Args: got %v, want empty (captured by FlagValues)", wrapper.Args)
	}
	if _, ok := wrapper.Flags["c"]; !ok {
		t.Errorf("wrapper.Flags should contain c, got %v", wrapper.Flags)
	}
	vals := wrapper.FlagValues["c"]
	if len(vals) != 1 || vals[0] != "rm -rf /etc/foo" {
		t.Errorf("wrapper.FlagValues[c]: got %v, want [\"rm -rf /etc/foo\"]", vals)
	}
}

func TestBashParser_RecursiveEval_DepthExceeded(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: map[string]*clispec.CLISpec{
			"bash": {Recurse: &clispec.RecurseSpec{Flags: []string{"c"}}},
		},
		MaxDepth: 2,
	}
	cmd := `bash -c "bash -c \"bash -c ls\""`
	raw, _ := json.Marshal(map[string]string{"command": cmd})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	last := cmds[len(cmds)-1]
	if !last.InspectionFailed {
		t.Errorf("deepest bash should have InspectionFailed: %+v", last)
	}
}

func TestBashParser_RecursiveEval_NilConfigBehavesLikeBefore(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{}
	raw, _ := json.Marshal(map[string]string{"command": `bash -c "rm -rf /"`})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "bash" {
		t.Errorf("nil config should not recurse: %v", cmds)
	}
}

func TestBashParser_ValueTakingFlags_SpaceForm(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		ValueTakingFlags: map[string]map[string]struct{}{
			"terraform": {"chdir": {}},
		},
	}
	raw, _ := json.Marshal(map[string]string{
		"command": "terraform -chdir environments/prod apply",
	})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("cmds: %d", len(cmds))
	}
	got := cmds[0].FlagValues["chdir"]
	if !reflect.DeepEqual(got, []string{"environments/prod"}) {
		t.Errorf("FlagValues[chdir]: got %v, want [environments/prod]", got)
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"apply"}) {
		t.Errorf("Args: got %v, want [apply]", cmds[0].Args)
	}
}

func TestBashParser_ValueTakingFlags_WildcardCommand(t *testing.T) {
	t.Parallel()
	// "" key applies to every command name (wildcard bucket).
	p := &parser.BashParser{
		ValueTakingFlags: map[string]map[string]struct{}{
			"": {"chdir": {}},
		},
	}
	raw, _ := json.Marshal(map[string]string{"command": "anycmd -chdir foo"})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := cmds[0].FlagValues["chdir"]; !reflect.DeepEqual(got, []string{"foo"}) {
		t.Errorf("wildcard set should apply: got %v", got)
	}
}

func TestBashParser_ValueTakingFlags_MissingValueMarksInspectionFailed(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		ValueTakingFlags: map[string]map[string]struct{}{
			"terraform": {"chdir": {}},
		},
	}
	raw, _ := json.Marshal(map[string]string{"command": "terraform -chdir"})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cmds[0].InspectionFailed {
		t.Errorf("InspectionFailed should be true when value is missing at end")
	}
}

func TestBashParser_DockerRunSubcommand(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: clispec.Builtins(),
		MaxDepth: 3,
	}
	raw, _ := json.Marshal(map[string]string{
		"command": "docker run -it --name x ubuntu rm -rf /",
	})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 cmds (docker + rm), got %d: %+v", len(cmds), cmds)
	}
	if cmds[0].Name != "docker" {
		t.Errorf("cmds[0].Name: %s", cmds[0].Name)
	}
	last := cmds[len(cmds)-1]
	if last.Name != "rm" {
		t.Errorf("last cmd should be rm, got %s", last.Name)
	}
}

func TestBashParser_KubectlExecSubcommand(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: clispec.Builtins(),
		MaxDepth: 3,
	}
	raw, _ := json.Marshal(map[string]string{
		"command": "kubectl exec pod -c sidecar rm -rf /etc",
	})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	last := cmds[len(cmds)-1]
	if last.Name != "rm" {
		t.Errorf("last cmd should be rm, got %s", last.Name)
	}
}

func TestBashParser_DockerRunTruncated_InspectionFailed(t *testing.T) {
	t.Parallel()
	p := &parser.BashParser{
		CLISpecs: clispec.Builtins(),
		MaxDepth: 3,
	}
	raw, _ := json.Marshal(map[string]string{"command": "docker run"})
	cmds, err := p.Parse("Bash", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) == 0 || !cmds[0].InspectionFailed {
		t.Errorf("expected InspectionFailed=true on truncated docker run, got %+v", cmds)
	}
}
