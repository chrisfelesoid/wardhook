package rule_test

import (
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func TestDefaults_ResolvedCLISpecs_Builtin(t *testing.T) {
	t.Parallel()
	d := rule.Defaults{}
	got := d.ResolvedCLISpecs()
	// Built-in entries should at minimum cover bash and sh.
	for _, name := range []string{"bash", "sh"} {
		spec, ok := got[name]
		if !ok || spec == nil {
			t.Errorf("builtin should include %q: %v", name, got)
			continue
		}
		if spec.Recurse == nil {
			t.Errorf("builtin %q should declare recurse: %+v", name, spec)
		}
	}
}

func TestDefaults_ResolvedCLISpecs_UserAddsEntry(t *testing.T) {
	t.Parallel()
	d := rule.Defaults{
		CLISpecs: map[string]*clispec.CLISpec{
			"mycli": {
				Recurse: &clispec.RecurseSpec{Flags: []string{"x"}},
			},
		},
	}
	got := d.ResolvedCLISpecs()
	if _, ok := got["mycli"]; !ok {
		t.Errorf("user entry mycli missing: %v", got)
	}
	// Built-ins still merged in.
	if _, ok := got["bash"]; !ok {
		t.Errorf("builtin bash should still be present after merge: %v", got)
	}
}

func TestDefaults_ResolvedCLISpecs_UserOverrideMergesWithBuiltin(t *testing.T) {
	t.Parallel()
	d := rule.Defaults{
		CLISpecs: map[string]*clispec.CLISpec{
			"bash": {
				Recurse: &clispec.RecurseSpec{Flags: []string{"x"}},
			},
		},
	}
	got := d.ResolvedCLISpecs()
	spec, ok := got["bash"]
	if !ok || spec == nil || spec.Recurse == nil {
		t.Fatalf("bash spec missing after merge: %+v", spec)
	}
	// Merge yields union of flags: built-in "c" plus user "x".
	hasC := false
	hasX := false
	for _, f := range spec.Recurse.Flags {
		if f == "c" {
			hasC = true
		}
		if f == "x" {
			hasX = true
		}
	}
	if !hasC || !hasX {
		t.Errorf("merged bash.recurse.flags should union builtin and user: %v", spec.Recurse.Flags)
	}
}

func TestDefaults_ResolvedRecursiveMaxDepth_Default(t *testing.T) {
	t.Parallel()
	d := rule.Defaults{}
	if got := d.ResolvedRecursiveMaxDepth(); got != 3 {
		t.Errorf("default: got %d, want 3", got)
	}
}

func TestDefaults_ResolvedRecursiveMaxDepth_UserOverride(t *testing.T) {
	t.Parallel()
	d := rule.Defaults{RecursiveMaxDepth: 5}
	if got := d.ResolvedRecursiveMaxDepth(); got != 5 {
		t.Errorf("user: got %d, want 5", got)
	}
}
