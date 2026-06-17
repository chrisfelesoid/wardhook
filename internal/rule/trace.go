package rule

import (
	"fmt"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
)

// Outcome describes how a single Rule reacted to a single Command.
type Outcome int

// Outcome values.
const (
	OutcomeNoMatch Outcome = iota
	OutcomeExcepted
	OutcomeMatched
)

// TraceEntry is the result of evaluating one Rule against one Command.
type TraceEntry struct {
	RuleName     string
	Tool         string
	Action       hook.Decision
	Outcome      Outcome
	ExceptDetail string
}

// CommandTrace bundles all TraceEntry values for a single Command.
type CommandTrace struct {
	Command          parser.Command
	Rules            []TraceEntry
	InspectionFailed bool
}

// Trace is the full per-command, per-rule trace plus the final
// aggregate decision and reason, matching what Evaluate would return.
type Trace struct {
	Commands []CommandTrace
	Final    hook.Decision
	Reason   string
}

// EvaluateTrace mirrors Evaluate but additionally records, for every
// Rule x Command pair, why the rule did or did not contribute to the
// decision. Final and Reason match Evaluate's return values.
func EvaluateTrace(cfg *Config, toolName string, cmds []parser.Command) Trace {
	t := Trace{Final: hook.DecisionAllow}
	for _, cmd := range cmds {
		var ruleTraces []TraceEntry
		dec, reason := evalCommandWithTrace(cfg, toolName, cmd, &ruleTraces)
		if cmd.InspectionFailed && rank(dec) < rank(hook.DecisionAsk) {
			dec = hook.DecisionAsk
			reason = fmt.Sprintf(
				`[wardhook] asked: inspection failed for %q`,
				cmd.Raw,
			)
		}
		if rank(dec) > rank(t.Final) {
			t.Final = dec
			t.Reason = reason
		}
		t.Commands = append(t.Commands, CommandTrace{
			Command:          cmd,
			Rules:            ruleTraces,
			InspectionFailed: cmd.InspectionFailed,
		})
	}
	return t
}

func formatExceptDetail(spec *MatchSpec) string {
	if spec == nil {
		return ""
	}
	if spec.Command != "" {
		return fmt.Sprintf("command %q", spec.Command)
	}
	if len(spec.FlagsAll) > 0 {
		return fmt.Sprintf("flags_all %v", spec.FlagsAll)
	}
	if len(spec.FlagsAny) > 0 {
		return fmt.Sprintf("flags_any %v", spec.FlagsAny)
	}
	if len(spec.SubcommandsAll) > 0 {
		return fmt.Sprintf("subcommands_all %s", formatSubcommandPaths(spec.SubcommandsAll))
	}
	if len(spec.SubcommandsAny) > 0 {
		return fmt.Sprintf("subcommands_any %s", formatSubcommandPaths(spec.SubcommandsAny))
	}
	if len(spec.FlagValues) > 0 {
		return fmt.Sprintf("flag_values[%s]", spec.FlagValues[0].Name)
	}
	if spec.Glob != nil && len(spec.Glob.Patterns) > 0 {
		return fmt.Sprintf("glob %q", spec.Glob.Patterns[0])
	}
	if spec.Regex != nil && len(spec.Regex.Patterns) > 0 {
		return fmt.Sprintf("regex %q", spec.Regex.Patterns[0])
	}
	return ""
}

// formatSubcommandPaths renders SubcommandPaths in either flat or
// nested form. When every path has length 1, the flat form is used
// (e.g. "[push fetch]") to preserve the look of legacy fixtures.
// Otherwise the nested form is used (e.g. "[[pr create] [issue list]]").
func formatSubcommandPaths(paths SubcommandPaths) string {
	allSingle := true
	for _, p := range paths {
		if len(p) != 1 {
			allSingle = false
			break
		}
	}
	if allSingle {
		flat := make([]string, len(paths))
		for i, p := range paths {
			flat[i] = p[0]
		}
		return fmt.Sprintf("%v", flat)
	}
	return fmt.Sprintf("%v", paths)
}
