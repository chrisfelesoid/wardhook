package rule_test

import (
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func mkCmd(name string, flags []string, args []string) parser.Command {
	fs := map[string]struct{}{}
	for _, f := range flags {
		fs[f] = struct{}{}
	}
	return parser.Command{Name: name, Flags: fs, Args: args}
}

// matchSpec is a local alias for the test-only export in export_test.go.
func matchSpec(spec *rule.MatchSpec, cmd parser.Command) bool {
	return rule.MatchSpecFn(spec, cmd)
}

func TestMatch_CommandEquality(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "rm"}
	if !matchSpec(&spec, mkCmd("rm", nil, nil)) {
		t.Error("rm should match")
	}
	if matchSpec(&spec, mkCmd("ls", nil, nil)) {
		t.Error("ls should not match rm")
	}
}

func TestMatch_EmptyCommandIsWildcard(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{FlagsAll: []string{"r"}}
	if !matchSpec(&spec, mkCmd("anything", []string{"r"}, nil)) {
		t.Error("empty command should match any name")
	}
}

func TestMatch_FlagsAll(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}}
	if !matchSpec(&spec, mkCmd("rm", []string{"r", "f", "v"}, nil)) {
		t.Error("rfv should satisfy flags_all=[r,f]")
	}
	if matchSpec(&spec, mkCmd("rm", []string{"r"}, nil)) {
		t.Error("r alone should not satisfy flags_all=[r,f]")
	}
}

func TestMatch_FlagsAny(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", FlagsAny: []string{"force", "f"}}
	if !matchSpec(&spec, mkCmd("git", []string{"force"}, nil)) {
		t.Error("force should satisfy flags_any")
	}
	if matchSpec(&spec, mkCmd("git", []string{"verbose"}, nil)) {
		t.Error("verbose should not satisfy flags_any=[force,f]")
	}
}

func TestMatch_FlagAliases(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command:     "rm",
		FlagsAll:    []string{"r"},
		FlagAliases: map[string][]string{"r": {"recursive"}},
	}
	// Command parsed without aliases reports the flag as "recursive".
	// matchSpec must re-canonicalize via spec.FlagAliases.
	if !matchSpec(&spec, mkCmd("rm", []string{"recursive"}, nil)) {
		t.Error("recursive should match flags_all=[r] via alias")
	}
}

func mkCmdWithValues(name string, values map[string][]string) parser.Command {
	return parser.Command{
		Name:       name,
		Flags:      map[string]struct{}{},
		FlagValues: values,
	}
}

func TestMatch_FlagValues_SingleEntryHit(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command: "terraform",
		FlagValues: []rule.FlagValueMatch{
			{Name: "chdir", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"environments/prod", "environments/prod/**"},
			}},
		},
	}
	cmd := mkCmdWithValues("terraform",
		map[string][]string{"chdir": {"environments/prod"}})
	if !matchSpec(&spec, cmd) {
		t.Error("environments/prod should match")
	}
}

func TestMatch_FlagValues_SingleEntryMiss(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		FlagValues: []rule.FlagValueMatch{
			{Name: "chdir", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"environments/prod*"},
			}},
		},
	}
	cmd := mkCmdWithValues("terraform",
		map[string][]string{"chdir": {"environments/dev"}})
	if matchSpec(&spec, cmd) {
		t.Error("environments/dev should not match prod glob")
	}
}

func TestMatch_FlagValues_FlagMissingIsMiss(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		FlagValues: []rule.FlagValueMatch{
			{Name: "chdir", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"*"},
			}},
		},
	}
	cmd := mkCmdWithValues("terraform", map[string][]string{})
	if matchSpec(&spec, cmd) {
		t.Error("absent flag should miss")
	}
}

func TestMatch_FlagValues_MultipleEntriesAreAND(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		FlagValues: []rule.FlagValueMatch{
			{Name: "namespace", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"prod*"},
			}},
			{Name: "context", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"*-prod"},
			}},
		},
	}
	hit := mkCmdWithValues("kubectl", map[string][]string{
		"namespace": {"prod"},
		"context":   {"east-prod"},
	})
	if !matchSpec(&spec, hit) {
		t.Error("both entries should AND-match")
	}
	miss := mkCmdWithValues("kubectl", map[string][]string{
		"namespace": {"prod"},
		"context":   {"east-dev"},
	})
	if matchSpec(&spec, miss) {
		t.Error("partial match should not AND-match")
	}
}

func TestMatch_FlagValues_MultiValuesAreOR(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		FlagValues: []rule.FlagValueMatch{
			{Name: "var", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"secret=*"},
			}},
		},
	}
	cmd := mkCmdWithValues("terraform", map[string][]string{
		"var": {"name=foo", "secret=xyz"},
	})
	if !matchSpec(&spec, cmd) {
		t.Error("any value matching any glob should satisfy")
	}
}

func TestMatch_FlagValues_HonorsFlagAliases(t *testing.T) {
	t.Parallel()
	// FlagValueMatch.Name is written as the canonical name.
	// Even when cmd.FlagValues stores an alt-name key, the matcher
	// canonicalizes both sides before lookup.
	spec := rule.MatchSpec{
		FlagAliases: map[string][]string{"c": {"chdir"}},
		FlagValues: []rule.FlagValueMatch{
			{Name: "c", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"prod"},
			}},
		},
	}
	cmd := mkCmdWithValues("terraform",
		map[string][]string{"chdir": {"prod"}})
	if !matchSpec(&spec, cmd) {
		t.Error("alt-name key should be canonicalized to c and match")
	}
}

func TestEvalGlobMatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		spec   *rule.GlobMatch
		inputs []string
		want   bool
	}{
		{
			name:   "nil spec passthrough",
			spec:   nil,
			inputs: []string{"anything"},
			want:   true,
		},
		{
			name:   "any mode, empty inputs",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"**"}},
			inputs: []string{},
			want:   false,
		},
		{
			name:   "all mode, empty inputs (vacuous-true not adopted)",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAll, Patterns: []string{"**"}},
			inputs: []string{},
			want:   false,
		},
		{
			name:   "any mode, single input hit",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"/etc/**"}},
			inputs: []string{"/etc/passwd"},
			want:   true,
		},
		{
			name:   "any mode, single input miss",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"/etc/**"}},
			inputs: []string{"/var/log"},
			want:   false,
		},
		{
			name:   "any mode, one of many matches",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"/etc/**"}},
			inputs: []string{"/etc/x", "/var/log"},
			want:   true,
		},
		{
			name:   "all mode, all match",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAll, Patterns: []string{"/tmp/**"}},
			inputs: []string{"/tmp/a", "/tmp/b"},
			want:   true,
		},
		{
			name:   "all mode, one fails",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAll, Patterns: []string{"/tmp/**"}},
			inputs: []string{"/tmp/a", "/etc/x"},
			want:   false,
		},
		{
			name:   "any mode, multiple patterns OR",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"/etc/**", "/usr/**"}},
			inputs: []string{"/usr/bin/ls"},
			want:   true,
		},
		{
			name:   "any mode, literal namespace style",
			spec:   &rule.GlobMatch{Mode: rule.GlobModeAny, Patterns: []string{"prod*"}},
			inputs: []string{"prod", "prod-app"},
			want:   true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := rule.EvalGlobMatchFn(c.spec, c.inputs)
			if got != c.want {
				t.Errorf("evalGlobMatch(%+v, %v): got %v, want %v", c.spec, c.inputs, got, c.want)
			}
		})
	}
}

func TestMatch_Glob_AnyMode_SingleArg(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Glob: &rule.GlobMatch{
			Mode:     rule.GlobModeAny,
			Patterns: []string{"**/.env"},
		},
	}
	if !matchSpec(&spec, mkCmd("", nil, []string{"/workspace/app/.env"})) {
		t.Error("/workspace/app/.env should match **/.env")
	}
	if matchSpec(&spec, mkCmd("", nil, []string{"/workspace/main.go"})) {
		t.Error("main.go should not match **/.env")
	}
}

func TestMatch_Glob_AllMode_RejectsMixedArgs(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Glob: &rule.GlobMatch{
			Mode:     rule.GlobModeAll,
			Patterns: []string{"/tmp/**"},
		},
	}
	safe := mkCmd("", nil, []string{"/tmp/x", "/tmp/y"})
	if !matchSpec(&spec, safe) {
		t.Error("all-args-in-tmp should match all-mode")
	}
	mixed := mkCmd("", nil, []string{"/tmp/x", "/etc/passwd"})
	if matchSpec(&spec, mixed) {
		t.Error("mixed args should NOT match all-mode")
	}
}

func TestMatch_Glob_NilFieldIsPassthrough(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "rm"} // no Glob
	if !matchSpec(&spec, mkCmd("rm", nil, []string{"/etc/passwd"})) {
		t.Error("nil Glob should not block match")
	}
}

func TestMatch_FlagValues_NewGlobMatch_Any(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command: "terraform",
		FlagValues: []rule.FlagValueMatch{
			{Name: "chdir", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAny,
				Patterns: []string{"environments/prod"},
			}},
		},
	}
	hit := mkCmdWithValues("terraform",
		map[string][]string{"chdir": {"environments/prod"}})
	if !matchSpec(&spec, hit) {
		t.Error("any mode should hit on prod")
	}
	miss := mkCmdWithValues("terraform",
		map[string][]string{"chdir": {"environments/dev"}})
	if matchSpec(&spec, miss) {
		t.Error("any mode should miss on dev")
	}
}

func TestMatch_FlagValues_NewGlobMatch_AllRejectsMixed(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command: "kubectl",
		FlagValues: []rule.FlagValueMatch{
			{Name: "n", Glob: &rule.GlobMatch{
				Mode:     rule.GlobModeAll,
				Patterns: []string{"dev*"},
			}},
		},
	}
	allDev := mkCmdWithValues("kubectl",
		map[string][]string{"n": {"dev", "dev-app"}})
	if !matchSpec(&spec, allDev) {
		t.Error("all-mode should match when every value is dev*")
	}
	mixed := mkCmdWithValues("kubectl",
		map[string][]string{"n": {"dev", "prod"}})
	if matchSpec(&spec, mixed) {
		t.Error("all-mode should reject mixed prod/dev")
	}
}

func TestMatchRegex(t *testing.T) {
	t.Parallel()
	mk := func(mode rule.GlobMode, patterns ...string) *rule.RegexMatch {
		r := &rule.RegexMatch{Mode: mode, Patterns: patterns}
		if err := r.Validate("test"); err != nil {
			t.Fatalf("validate: %v", err)
		}
		return r
	}

	cases := []struct {
		name   string
		spec   *rule.RegexMatch
		inputs []string
		want   bool
	}{
		{"nil spec passthrough", nil, []string{"x"}, true},
		{"any empty inputs", mk(rule.GlobModeAny, ".*"), nil, false},
		{"all empty inputs (fail-closed)", mk(rule.GlobModeAll, ".*"), nil, false},
		{"any 1 hit", mk(rule.GlobModeAny, "^777$"), []string{"777"}, true},
		{"any 1 miss", mk(rule.GlobModeAny, "^777$"), []string{"755"}, false},
		{"any multi-pattern OR", mk(rule.GlobModeAny, "^777$", `^a\+rwx$`), []string{"a+rwx"}, true},
		{"all uniform", mk(rule.GlobModeAll, `^[0-9]+$`), []string{"777", "755"}, true},
		{"all mixed", mk(rule.GlobModeAll, `^[0-9]+$`), []string{"777", "abc"}, false},
		{
			"chmod-style anchored",
			mk(rule.GlobModeAny, `^[0-7]?777$`, `^[augo]*\+[rwx]+$`),
			[]string{"777", "/etc/passwd"},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := rule.MatchRegexFn(c.spec, c.inputs)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestMatch_RegexAndGlob_AND(t *testing.T) {
	t.Parallel()
	regex := &rule.RegexMatch{
		Mode:     rule.GlobModeAny,
		Patterns: []string{`^[0-7]?777$`},
	}
	if err := regex.Validate("test"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	spec := rule.MatchSpec{
		Command: "chmod",
		Glob: &rule.GlobMatch{
			Mode:     rule.GlobModeAny,
			Patterns: []string{"/etc/**"},
		},
		Regex: regex,
	}
	// both hit → match
	if !matchSpec(&spec, mkCmd("chmod", nil, []string{"777", "/etc/passwd"})) {
		t.Error("777 + /etc/passwd should match (both glob and regex hit)")
	}
	// regex hit but glob miss → no match
	if matchSpec(&spec, mkCmd("chmod", nil, []string{"777", "/tmp/safe"})) {
		t.Error("777 + /tmp/safe should not match (glob miss)")
	}
	// glob hit but regex miss → no match
	if matchSpec(&spec, mkCmd("chmod", nil, []string{"755", "/etc/hosts"})) {
		t.Error("755 + /etc/hosts should not match (regex miss)")
	}
}

func TestMatch_FlagValues_Regex(t *testing.T) {
	t.Parallel()
	r := &rule.RegexMatch{
		Mode:     rule.GlobModeAny,
		Patterns: []string{`^prod(-\d+)?$`},
	}
	if err := r.Validate("test"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	spec := rule.MatchSpec{
		Command: "kubectl",
		FlagValues: []rule.FlagValueMatch{
			{Name: "n", Regex: r},
		},
	}
	// regex hit
	hit := mkCmdWithValues("kubectl", map[string][]string{"n": {"prod-42"}})
	if !matchSpec(&spec, hit) {
		t.Error("prod-42 should match ^prod(-\\d+)?$")
	}
	// regex miss
	miss := mkCmdWithValues("kubectl", map[string][]string{"n": {"dev"}})
	if matchSpec(&spec, miss) {
		t.Error("dev should not match")
	}
}

func TestMatch_FlagValues_GlobAndRegex_AND(t *testing.T) {
	t.Parallel()
	r := &rule.RegexMatch{
		Mode:     rule.GlobModeAll,
		Patterns: []string{`^[A-Z_]+=[^/]+$`},
	}
	if err := r.Validate("test"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	spec := rule.MatchSpec{
		FlagValues: []rule.FlagValueMatch{
			{
				Name: "var",
				Glob: &rule.GlobMatch{
					Mode:     rule.GlobModeAll,
					Patterns: []string{"*=*"},
				},
				Regex: r,
			},
		},
	}
	// both hit
	hit := mkCmdWithValues("", map[string][]string{
		"var": {"DB_HOST=localhost", "DB_PORT=5432"},
	})
	if !matchSpec(&spec, hit) {
		t.Error("both DB_HOST=localhost and DB_PORT=5432 should match both glob and regex")
	}
	// glob ok but regex miss (one value has path '/')
	miss := mkCmdWithValues("", map[string][]string{
		"var": {"DB_HOST=localhost", "path=/etc/passwd"},
	})
	if matchSpec(&spec, miss) {
		t.Error("path=/etc/passwd should fail regex ^[A-Z_]+=[^/]+$")
	}
}

func TestMatch_SubcommandsAny_SingleMatch(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"push"}}
	if !matchSpec(&spec, mkCmd("git", nil, []string{"push", "origin", "main"})) {
		t.Error("git push origin main should match subcommands_any=[push]")
	}
}

func TestMatch_SubcommandsAny_MultiOption(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"push", "fetch", "pull"}}
	if !matchSpec(&spec, mkCmd("git", nil, []string{"fetch", "upstream"})) {
		t.Error("git fetch upstream should match subcommands_any=[push fetch pull]")
	}
}

func TestMatch_SubcommandsAny_NoMatch(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"push"}}
	if matchSpec(&spec, mkCmd("git", nil, []string{"status"})) {
		t.Error("git status should not match subcommands_any=[push]")
	}
}

func TestMatch_SubcommandsAny_EmptyArgs(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"push"}}
	if matchSpec(&spec, mkCmd("git", nil, nil)) {
		t.Error("git (no args) must not match subcommands_any=[push] (fail-closed)")
	}
}

func TestMatch_SubcommandsAny_EmptySpec_Passthrough(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git"}
	if !matchSpec(&spec, mkCmd("git", nil, []string{"status"})) {
		t.Error("empty subcommands_any should be passthrough (match by command alone)")
	}
}

func TestMatch_SubcommandsAny_CaseSensitive(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"Push"}}
	if matchSpec(&spec, mkCmd("git", nil, []string{"push"})) {
		t.Error("case-sensitive comparison: Push must not match push")
	}
}

func TestMatch_SubcommandsAll_SingleMatch(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAll: []string{"push"}}
	if !matchSpec(&spec, mkCmd("git", nil, []string{"push", "origin"})) {
		t.Error("git push origin should match subcommands_all=[push]")
	}
}

func TestMatch_SubcommandsAll_EmptyArgs(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAll: []string{"push"}}
	if matchSpec(&spec, mkCmd("git", nil, nil)) {
		t.Error("git (no args) must not match subcommands_all=[push] (fail-closed)")
	}
}

func TestMatch_SubcommandsAll_MultipleWants_AlwaysFalse(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAll: []string{"push", "fetch"}}
	if matchSpec(&spec, mkCmd("git", nil, []string{"push"})) {
		t.Error("subcommands_all=[push fetch] cannot match a single Args[0]")
	}
}

func TestMatch_SubcommandsAny_AndedWithCommand(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{Command: "git", SubcommandsAny: []string{"push"}}
	if matchSpec(&spec, mkCmd("docker", nil, []string{"push", "img"})) {
		t.Error("command=git is AND, so docker push should not match")
	}
}

func TestMatch_SubcommandsAny_AndedWithFlags(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command:        "git",
		SubcommandsAny: []string{"push"},
		FlagsAny:       []string{"force"},
	}
	if !matchSpec(&spec, mkCmd("git", []string{"force"}, []string{"push", "origin"})) {
		t.Error("git push --force should match subcommands_any+flags_any")
	}
	if matchSpec(&spec, mkCmd("git", nil, []string{"push", "origin"})) {
		t.Error("git push (no --force) should not match because flags_any is AND'd")
	}
}

func TestMatch_SubcommandsAnyAndAll_AndedTogether(t *testing.T) {
	t.Parallel()
	spec := rule.MatchSpec{
		Command:        "git",
		SubcommandsAll: []string{"push"},
		SubcommandsAny: []string{"push", "fetch"},
	}
	if !matchSpec(&spec, mkCmd("git", nil, []string{"push"})) {
		t.Error("both clauses satisfied by push")
	}
	if matchSpec(&spec, mkCmd("git", nil, []string{"fetch"})) {
		t.Error("subcommands_all=[push] requires Args[0]==push, so fetch must fail")
	}
}
