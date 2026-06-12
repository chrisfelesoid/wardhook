package rule_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func cfg(rules ...rule.Rule) *rule.Config {
	return &rule.Config{Version: 1, Rules: rules}
}

func TestEvaluate_NoRuleMatched_Allows(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{Name: "r1", Tool: "Bash",
		Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionDeny})
	d, _ := rule.Evaluate(c, "Bash", []parser.Command{mkCmd("ls", nil, nil)})
	if d != hook.DecisionAllow {
		t.Errorf("got %q, want allow", d)
	}
}

func TestEvaluate_DenyBeatsAllow(t *testing.T) {
	t.Parallel()
	c := cfg(
		rule.Rule{Name: "a", Tool: "Bash", Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionAllow},
		rule.Rule{Name: "d", Tool: "Bash", Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionDeny},
	)
	d, reason := rule.Evaluate(c, "Bash", []parser.Command{mkCmd("rm", nil, nil)})
	if d != hook.DecisionDeny {
		t.Errorf("decision: %q", d)
	}
	if !strings.Contains(reason, "d") {
		t.Errorf("reason should mention rule d: %q", reason)
	}
}

func TestEvaluate_AskBeatsAllow(t *testing.T) {
	t.Parallel()
	c := cfg(
		rule.Rule{Name: "a", Tool: "Bash", Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionAllow},
		rule.Rule{Name: "k", Tool: "Bash", Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionAsk},
	)
	d, _ := rule.Evaluate(c, "Bash", []parser.Command{mkCmd("rm", nil, nil)})
	if d != hook.DecisionAsk {
		t.Errorf("decision: %q", d)
	}
}

func TestEvaluate_WildcardTool(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{Name: "secret", Tool: "*",
		Match: rule.MatchSpec{Glob: &rule.GlobMatch{
			Mode:     rule.GlobModeAny,
			Patterns: []string{"**/.env"},
		}}, Action: hook.DecisionDeny})
	d, _ := rule.Evaluate(c, "Read", []parser.Command{mkCmd("", nil, []string{"/proj/.env"})})
	if d != hook.DecisionDeny {
		t.Errorf("expected deny for wildcard rule on Read tool, got %q", d)
	}
}

func TestEvaluate_ToolMismatchIsIgnored(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{Name: "r", Tool: "Bash",
		Match: rule.MatchSpec{Command: "rm"}, Action: hook.DecisionDeny})
	d, _ := rule.Evaluate(c, "Read", []parser.Command{mkCmd("rm", nil, nil)})
	if d != hook.DecisionAllow {
		t.Errorf("Bash rule should not apply to Read tool: got %q", d)
	}
}

func TestEvaluate_ExceptExemptsMatch(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm-rf", Tool: "Bash",
		Match: rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Except: &rule.MatchSpec{Glob: &rule.GlobMatch{
			Mode:     rule.GlobModeAll,
			Patterns: []string{"/tmp/**"},
		}},
		Action: hook.DecisionDeny,
	})
	d, _ := rule.Evaluate(c, "Bash",
		[]parser.Command{mkCmd("rm", []string{"r", "f"}, []string{"/tmp/x"})})
	if d != hook.DecisionAllow {
		t.Errorf("expected allow via except, got %q", d)
	}
}

func TestEvaluate_MultiCommandAggregates(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{Name: "block-rm-rf", Tool: "Bash",
		Match:  rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Action: hook.DecisionDeny})
	cmds := []parser.Command{
		mkCmd("echo", nil, []string{"hi"}),
		mkCmd("rm", []string{"r", "f"}, []string{"/etc/foo"}),
	}
	d, _ := rule.Evaluate(c, "Bash", cmds)
	if d != hook.DecisionDeny {
		t.Errorf("overall: %q", d)
	}
}

func TestEvaluate_InspectionFailedDegradesToAsk(t *testing.T) {
	t.Parallel()
	c := cfg() // no rules
	cmds := []parser.Command{
		{Name: "bash", Raw: `bash -c "broken`, InspectionFailed: true},
	}
	dec, reason := rule.Evaluate(c, "Bash", cmds)
	if dec != hook.DecisionAsk {
		t.Errorf("decision: got %q, want ask", dec)
	}
	if !strings.Contains(reason, "inspection failed") {
		t.Errorf("reason should mention inspection failure: %q", reason)
	}
}

func TestEvaluate_InspectionFailedDoesNotOverrideDeny(t *testing.T) {
	t.Parallel()
	c := cfg(rule.Rule{
		Name: "block-rm-rf", Tool: "Bash",
		Match:  rule.MatchSpec{Command: "rm", FlagsAll: []string{"r", "f"}},
		Action: hook.DecisionDeny,
	})
	cmds := []parser.Command{
		{Name: "bash", Raw: `bash -c "broken`, InspectionFailed: true},
		mkCmd("rm", []string{"r", "f"}, []string{"/etc/foo"}),
	}
	dec, _ := rule.Evaluate(c, "Bash", cmds)
	if dec != hook.DecisionDeny {
		t.Errorf("decision: got %q, want deny", dec)
	}
}
