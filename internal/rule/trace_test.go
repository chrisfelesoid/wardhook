package rule_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func TestFormatExceptDetail_PicksFirstNonEmptyClause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		spec rule.MatchSpec
		want string
	}{
		{
			name: "command takes precedence",
			spec: rule.MatchSpec{
				Command:  "rm",
				FlagsAll: []string{"r"},
			},
			want: `command "rm"`,
		},
		{
			name: "flags_all when no command",
			spec: rule.MatchSpec{FlagsAll: []string{"r", "f"}},
			want: "flags_all [r f]",
		},
		{
			name: "flags_any when no command or flags_all",
			spec: rule.MatchSpec{FlagsAny: []string{"force"}},
			want: "flags_any [force]",
		},
		{
			name: "subcommands_all when no command/flags",
			spec: rule.MatchSpec{SubcommandsAll: rule.SubcommandPaths{{"push"}}},
			want: "subcommands_all [push]",
		},
		{
			name: "subcommands_any when no command/flags/subcommands_all",
			spec: rule.MatchSpec{SubcommandsAny: rule.SubcommandPaths{{"status"}, {"log"}}},
			want: "subcommands_any [status log]",
		},
		{
			name: "flags_any precedes subcommands_any (existing order preserved)",
			spec: rule.MatchSpec{
				FlagsAny:       []string{"force"},
				SubcommandsAny: rule.SubcommandPaths{{"push"}},
			},
			want: "flags_any [force]",
		},
		{
			name: "glob first pattern",
			spec: rule.MatchSpec{Glob: &rule.GlobMatch{
				Mode: rule.GlobModeAll, Patterns: []string{"/tmp/**", "**/build/**"},
			}},
			want: `glob "/tmp/**"`,
		},
		{
			name: "regex first pattern",
			spec: rule.MatchSpec{Regex: &rule.RegexMatch{
				Mode: rule.GlobModeAny, Patterns: []string{"^777$"},
			}},
			want: `regex "^777$"`,
		},
		{
			name: "empty spec produces empty string",
			spec: rule.MatchSpec{},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := rule.FormatExceptDetailForTest(&c.spec)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestEvaluateTrace_SingleRuleMatch(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm", Tool: "Bash",
		Match:  rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Action: hook.DecisionDeny,
	})
	tr := rule.EvaluateTrace(c, "Bash",
		[]parser.Command{mkCmd("rm", []string{"r", "f"}, []string{"/etc/foo"})})
	if tr.Final != hook.DecisionDeny {
		t.Fatalf("final: %q", tr.Final)
	}
	if len(tr.Commands) != 1 || len(tr.Commands[0].Rules) != 1 {
		t.Fatalf("trace shape: %+v", tr)
	}
	rt := tr.Commands[0].Rules[0]
	if rt.Outcome != rule.OutcomeMatched || rt.RuleName != "block-rm" {
		t.Errorf("rule trace: %+v", rt)
	}
}

func TestEvaluateTrace_NoMatchReportsNoMatch(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm", Tool: "Bash",
		Match:  rule.MatchSpec{Command: "rm"},
		Action: hook.DecisionDeny,
	})
	tr := rule.EvaluateTrace(c, "Bash",
		[]parser.Command{mkCmd("ls", nil, nil)})
	if tr.Final != hook.DecisionAllow {
		t.Fatalf("final: %q", tr.Final)
	}
	if tr.Commands[0].Rules[0].Outcome != rule.OutcomeNoMatch {
		t.Errorf("outcome: %v", tr.Commands[0].Rules[0].Outcome)
	}
}

func TestEvaluateTrace_ExceptRecordsDetail(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm", Tool: "Bash",
		Match: rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Except: &rule.MatchSpec{Glob: &rule.GlobMatch{
			Mode: rule.GlobModeAll, Patterns: []string{"/tmp/**"},
		}},
		Action: hook.DecisionDeny,
	})
	tr := rule.EvaluateTrace(c, "Bash",
		[]parser.Command{mkCmd("rm", []string{"r", "f"}, []string{"/tmp/foo"})})
	if tr.Final != hook.DecisionAllow {
		t.Fatalf("final: %q", tr.Final)
	}
	rt := tr.Commands[0].Rules[0]
	if rt.Outcome != rule.OutcomeExcepted {
		t.Errorf("outcome: %v", rt.Outcome)
	}
	if !strings.Contains(rt.ExceptDetail, `"/tmp/**"`) {
		t.Errorf("except detail: %q", rt.ExceptDetail)
	}
}

func TestEvaluateTrace_DenyBeatsAskAggregation(t *testing.T) {
	t.Parallel()
	c := cfg(
		rule.Rule{Name: "ask-rm", Tool: "Bash",
			Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionAsk},
		rule.Rule{Name: "deny-rm-rf", Tool: "Bash",
			Match:  rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
			Action: hook.DecisionDeny},
	)
	tr := rule.EvaluateTrace(c, "Bash",
		[]parser.Command{mkCmd("rm", []string{"r", "f"}, []string{"/etc/foo"})})
	if tr.Final != hook.DecisionDeny {
		t.Errorf("final: %q", tr.Final)
	}
	if len(tr.Commands[0].Rules) != 2 {
		t.Errorf("expected both rules in trace, got %d", len(tr.Commands[0].Rules))
	}
}

func TestEvaluateTrace_MultiCommandKeepsPerCommandTrace(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm-rf", Tool: "Bash",
		Match:  rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Action: hook.DecisionDeny,
	})
	cmds := []parser.Command{
		mkCmd("echo", nil, []string{"hi"}),
		mkCmd("rm", []string{"r", "f"}, []string{"/etc/foo"}),
	}
	tr := rule.EvaluateTrace(c, "Bash", cmds)
	if tr.Final != hook.DecisionDeny {
		t.Errorf("final: %q", tr.Final)
	}
	if len(tr.Commands) != 2 {
		t.Fatalf("command traces: %d", len(tr.Commands))
	}
	if tr.Commands[0].Rules[0].Outcome != rule.OutcomeNoMatch {
		t.Errorf("first command should miss: %+v", tr.Commands[0].Rules[0])
	}
	if tr.Commands[1].Rules[0].Outcome != rule.OutcomeMatched {
		t.Errorf("second command should match: %+v", tr.Commands[1].Rules[0])
	}
}

func TestEvaluateTrace_InspectionFailedDegradesToAsk(t *testing.T) {
	t.Parallel()
	c := cfg()
	cmds := []parser.Command{
		{Name: "bash", Raw: `bash -c "broken`, InspectionFailed: true},
	}
	tr := rule.EvaluateTrace(c, "Bash", cmds)
	if tr.Final != hook.DecisionAsk {
		t.Errorf("final: %q", tr.Final)
	}
	if !tr.Commands[0].InspectionFailed {
		t.Errorf("command trace should mark inspection_failed")
	}
	if !strings.Contains(tr.Reason, "inspection failed") {
		t.Errorf("reason should mention inspection failed: %q", tr.Reason)
	}
}
