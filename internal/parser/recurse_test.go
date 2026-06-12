package parser //nolint:testpackage // tests unexported helper

import (
	"reflect"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
)

func parseCall(t *testing.T, command string) *syntax.CallExpr {
	t.Helper()
	p := syntax.NewParser()
	file, err := p.Parse(strings.NewReader(command), "test")
	if err != nil {
		t.Fatalf("parse %q: %v", command, err)
	}
	var found *syntax.CallExpr
	syntax.Walk(file, func(node syntax.Node) bool {
		if c, ok := node.(*syntax.CallExpr); ok && found == nil {
			found = c
		}
		return true
	})
	if found == nil {
		t.Fatalf("no CallExpr in %q", command)
	}
	return found
}

func TestExtractRecursionTargets_ShortFlag(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `bash -c "rm -rf /"`)
	spec := &clispec.RecurseSpec{Flags: []string{"c"}}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed")
	}
	if len(targets) != 1 || targets[0] != "rm -rf /" {
		t.Errorf("targets: %v", targets)
	}
}

func TestExtractRecursionTargets_LongFlagEquals(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud --command="ls -l"`)
	spec := &clispec.RecurseSpec{Flags: []string{"command"}}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed")
	}
	if len(targets) != 1 || targets[0] != "ls -l" {
		t.Errorf("targets: %v", targets)
	}
}

func TestExtractRecursionTargets_LongFlagSeparated(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud --command "ls -l"`)
	spec := &clispec.RecurseSpec{Flags: []string{"command"}}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed")
	}
	if len(targets) != 1 || targets[0] != "ls -l" {
		t.Errorf("targets: %v", targets)
	}
}

func TestExtractRecursionTargets_DoubleDash(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud compute ssh my-vm -- rm -rf /tmp/x`)
	spec := &clispec.RecurseSpec{Terminator: true}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed")
	}
	if len(targets) != 1 || targets[0] != "rm -rf /tmp/x" {
		t.Errorf("targets: %v", targets)
	}
}

func TestExtractRecursionTargets_DoubleDash_EmptyAfter(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud compute ssh my-vm --`)
	spec := &clispec.RecurseSpec{Terminator: true}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf(`empty after "--" must not be failed`)
	}
	if len(targets) != 0 {
		t.Errorf("targets should be empty: %v", targets)
	}
}

func TestExtractRecursionTargets_FlagMissingValue(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `bash -c`)
	spec := &clispec.RecurseSpec{Flags: []string{"c"}}
	_, failed := extractRecursionTargets(call, spec, nil)
	if !failed {
		t.Errorf("missing flag value should be failed")
	}
}

func TestExtractRecursionTargets_EmptyEqualsValue(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud --command=`)
	spec := &clispec.RecurseSpec{Flags: []string{"command"}}
	_, failed := extractRecursionTargets(call, spec, nil)
	if !failed {
		t.Errorf(`--command= with empty value should be failed`)
	}
}

func TestExtractRecursionTargets_NotMatched(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `echo hi`)
	spec := &clispec.RecurseSpec{Flags: []string{"c"}}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed for unrelated command")
	}
	if len(targets) != 0 {
		t.Errorf("targets should be empty: %v", targets)
	}
}

func TestExtractRecursionTargets_MultipleKeys(t *testing.T) {
	t.Parallel()
	call := parseCall(t, `gcloud --command="ls" compute ssh -- rm -rf /tmp`)
	spec := &clispec.RecurseSpec{Flags: []string{"command"}, Terminator: true}
	targets, failed := extractRecursionTargets(call, spec, nil)
	if failed {
		t.Errorf("unexpected failed")
	}
	if len(targets) != 2 {
		t.Fatalf("targets: %v", targets)
	}
	if targets[0] != "ls" {
		t.Errorf("targets[0]: %q", targets[0])
	}
	if targets[1] != "rm -rf /tmp" {
		t.Errorf("targets[1]: %q", targets[1])
	}
}

func TestExtractSubcommandTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		command     string
		valueTaking map[string]struct{}
		subcommands map[string]*clispec.SubcommandRecurse
		wantStart   int
		wantFound   bool
		wantOK      bool
	}{
		{
			name:        "docker run -it --name x ubuntu rm -rf /",
			command:     "docker run -it --name my-ubuntu ubuntu rm -rf /",
			valueTaking: map[string]struct{}{"name": {}},
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}},
			wantStart:   6, // index of "rm"
			wantFound:   true, wantOK: true,
		},
		{
			name:        "docker run --rm -it ubuntu rm -rf /",
			command:     "docker run --rm -it ubuntu rm -rf /",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}},
			wantStart:   5, // index of "rm" (inner command)
			wantFound:   true, wantOK: true,
		},
		{
			name:        "docker exec ct -- rm -rf /",
			command:     "docker exec ct -- rm -rf /",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"exec": {Skip: 1}},
			// After exec (pos 0) + skip 1 (ct, pos 1) → next index = 3 ("--")
			wantStart: 3,
			wantFound: true, wantOK: true,
		},
		{
			name:        "docker logs container (verb not in map)",
			command:     "docker logs container",
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}, "exec": {Skip: 1}},
			wantFound:   false,
		},
		{
			name:        "docker run -it (skip target missing)",
			command:     "docker run -it",
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}},
			wantFound:   true, wantOK: false,
		},
		{
			name:        "kubectl exec pod -c sidecar curl evil.com",
			command:     "kubectl exec pod -c sidecar curl evil.com",
			valueTaking: map[string]struct{}{"c": {}},
			subcommands: map[string]*clispec.SubcommandRecurse{"exec": {Skip: 1}},
			wantStart:   5, // index of "curl"
			wantFound:   true, wantOK: true,
		},
		{
			name:        "kubectl exec pod cmd (skip=1 lands on cmd)",
			command:     "kubectl exec pod cmd",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"exec": {Skip: 1}},
			wantStart:   3, // index of "cmd"
			wantFound:   true, wantOK: true,
		},
		{
			name:        "nsenter --target 1234 cmd (no subcommands)",
			command:     "nsenter --target 1234 cmd",
			valueTaking: map[string]struct{}{"target": {}},
			subcommands: nil,
			wantFound:   false,
		},
		{
			name:        "docker run with -- in middle",
			command:     "docker run ubuntu -- rm -rf /",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}},
			// run + skip 1 (ubuntu) → next index = 3 ("--")
			wantStart: 3,
			wantFound: true, wantOK: true,
		},
		{
			name:        "skip 0 means immediately recurse",
			command:     "wrapper subcmd cmd arg",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"subcmd": {Skip: 0}},
			wantStart:   2, // index of "cmd"
			wantFound:   true, wantOK: true,
		},
		{
			name:        "docker run -- (--- before skip satisfied → InspectionFailed)",
			command:     "docker run --",
			valueTaking: map[string]struct{}{},
			subcommands: map[string]*clispec.SubcommandRecurse{"run": {Skip: 1}},
			wantFound:   true, wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			words := strings.Split(c.command, " ")
			start, found, ok := extractSubcommandTarget(words, c.valueTaking, c.subcommands)
			if found != c.wantFound {
				t.Errorf("found: got %v, want %v", found, c.wantFound)
			}
			if ok != c.wantOK {
				t.Errorf("ok: got %v, want %v", ok, c.wantOK)
			}
			if c.wantFound && c.wantOK && start != c.wantStart {
				t.Errorf("start: got %d, want %d (words=%v)", start, c.wantStart, words)
			}
			_ = reflect.DeepEqual // keep import alive in case future cases add slice compares
		})
	}
}

func TestExtractRecursionTargets_SubcommandStrategy(t *testing.T) {
	t.Parallel()
	dockerSpec := &clispec.RecurseSpec{
		Terminator: true,
		Subcommands: map[string]*clispec.SubcommandRecurse{
			"run":  {Skip: 1},
			"exec": {Skip: 1},
		},
	}

	cases := []struct {
		name        string
		command     string
		spec        *clispec.RecurseSpec
		valueTaking map[string]struct{}
		wantInner   []string
		wantFailed  bool
	}{
		{
			name:      "docker run image cmd",
			command:   "docker run ubuntu rm -rf /",
			spec:      dockerSpec,
			wantInner: []string{"rm -rf /"},
		},
		{
			name:    "docker run with -- separator (both strategies fire)",
			command: "docker run ubuntu -- rm -rf /",
			spec:    dockerSpec,
			// subcommand: words[3:] starts at "--" → after stripping leading "--" → "rm -rf /"
			// terminator: words after "--" → "rm -rf /"
			wantInner: []string{"rm -rf /", "rm -rf /"},
		},
		{
			name:      "docker exec ct cmd",
			command:   "docker exec ct rm -rf /etc",
			spec:      dockerSpec,
			wantInner: []string{"rm -rf /etc"},
		},
		{
			name:      "docker logs container (no subcommand match)",
			command:   "docker logs container",
			spec:      dockerSpec,
			wantInner: nil,
		},
		{
			name:       "docker run (truncated → inspection failed)",
			command:    "docker run",
			spec:       dockerSpec,
			wantInner:  nil,
			wantFailed: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			words := strings.Split(c.command, " ")
			call := buildCallExpr(t, words)
			targets, failed := extractRecursionTargets(call, c.spec, c.valueTaking)
			if failed != c.wantFailed {
				t.Errorf("failed: got %v, want %v", failed, c.wantFailed)
			}
			if !reflect.DeepEqual(targets, c.wantInner) {
				t.Errorf("targets:\n got  %#v\n want %#v", targets, c.wantInner)
			}
		})
	}
}

// buildCallExpr is a small helper that constructs a syntax.CallExpr
// for testing. Uses mvdan.cc/sh/v3 to parse a synthetic command.
func buildCallExpr(t *testing.T, words []string) *syntax.CallExpr {
	t.Helper()
	src := strings.Join(words, " ")
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(src), "test")
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	var found *syntax.CallExpr
	syntax.Walk(file, func(n syntax.Node) bool {
		if call, ok := n.(*syntax.CallExpr); ok && found == nil {
			found = call
			return false
		}
		return true
	})
	if found == nil {
		t.Fatalf("no CallExpr in %q", src)
	}
	return found
}
