package clispec_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
)

func mergeTestBase() map[string]*clispec.CLISpec {
	return map[string]*clispec.CLISpec{
		"docker": {
			ValueTakingFlags: []string{"name", "volume"},
			Recurse: &clispec.RecurseSpec{
				Terminator: true,
				Subcommands: map[string]*clispec.SubcommandRecurse{
					"run":  {Skip: 1},
					"exec": {Skip: 1},
				},
			},
		},
		"bash": {
			Recurse: &clispec.RecurseSpec{Flags: []string{"c"}},
		},
	}
}

func TestMergeCLISpecs_UserNilReturnsBaseUnchanged(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	got := clispec.MergeCLISpecs(base, nil)
	if len(got) != 2 {
		t.Errorf("len: %d", len(got))
	}
	if got["docker"].Recurse.Subcommands["run"].Skip != 1 {
		t.Errorf("docker.run.skip changed")
	}
}

func TestMergeCLISpecs_UserAddsNewCLI(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"mycli": {ValueTakingFlags: []string{"profile"}},
	}
	got := clispec.MergeCLISpecs(base, user)
	if _, ok := got["mycli"]; !ok {
		t.Errorf("mycli missing")
	}
	if _, ok := got["docker"]; !ok {
		t.Errorf("docker missing")
	}
}

func TestMergeCLISpecs_UserExtendsValueTakingFlags(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"docker": {
			ValueTakingFlags: []string{"name", "label"}, // name overlaps
		},
	}
	got := clispec.MergeCLISpecs(base, user)
	flags := got["docker"].ValueTakingFlags
	sort.Strings(flags)
	want := []string{"label", "name", "volume"}
	if !reflect.DeepEqual(flags, want) {
		t.Errorf("value_taking_flags: got %v, want %v", flags, want)
	}
}

func TestMergeCLISpecs_UserExtendsRecurseFlags(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"bash": {
			Recurse: &clispec.RecurseSpec{Flags: []string{"c", "x"}},
		},
	}
	got := clispec.MergeCLISpecs(base, user)
	flags := got["bash"].Recurse.Flags
	sort.Strings(flags)
	want := []string{"c", "x"}
	if !reflect.DeepEqual(flags, want) {
		t.Errorf("flags: got %v, want %v", flags, want)
	}
}

func TestMergeCLISpecs_UserTerminatorORBase(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"bash": {
			Recurse: &clispec.RecurseSpec{Terminator: true},
		},
	}
	got := clispec.MergeCLISpecs(base, user)
	if !got["bash"].Recurse.Terminator {
		t.Errorf("bash.terminator should be true")
	}
}

func TestMergeCLISpecs_UserSubcommandOverridePlusBaseKept(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"docker": {
			Recurse: &clispec.RecurseSpec{
				Subcommands: map[string]*clispec.SubcommandRecurse{
					"run":   {Skip: 2}, // override
					"build": {Skip: 1}, // new
				},
			},
		},
	}
	got := clispec.MergeCLISpecs(base, user)
	if got["docker"].Recurse.Subcommands["run"].Skip != 2 {
		t.Errorf("run.skip should be overridden to 2")
	}
	if got["docker"].Recurse.Subcommands["exec"].Skip != 1 {
		t.Errorf("exec.skip should be kept from base")
	}
	if got["docker"].Recurse.Subcommands["build"].Skip != 1 {
		t.Errorf("build.skip should be added")
	}
}

func TestMergeCLISpecs_DoesNotMutateBase(t *testing.T) {
	t.Parallel()
	base := mergeTestBase()
	user := map[string]*clispec.CLISpec{
		"docker": {ValueTakingFlags: []string{"label"}},
	}
	_ = clispec.MergeCLISpecs(base, user)
	// base["docker"].ValueTakingFlags should still be [name, volume]
	if len(base["docker"].ValueTakingFlags) != 2 {
		t.Errorf("base mutated: %v", base["docker"].ValueTakingFlags)
	}
}
