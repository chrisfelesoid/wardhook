package rule

import (
	"fmt"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
)

// Evaluate applies cfg.Rules to each command and aggregates the
// resulting decisions using priority deny > ask > allow. The returned
// reason is the human-readable explanation for the winning rule.
//
// A Command with InspectionFailed=true is forced to at least ask so
// that uninspectable wrappers (bash -c "<broken>", depth exceeded,
// missing flag value) cannot slip through with allow.
func Evaluate(cfg *Config, toolName string, cmds []parser.Command) (hook.Decision, string) {
	overall := hook.DecisionAllow
	overallReason := ""
	for _, cmd := range cmds {
		dec, reason := evalCommand(cfg, toolName, cmd)
		if cmd.InspectionFailed && rank(dec) < rank(hook.DecisionAsk) {
			dec = hook.DecisionAsk
			reason = fmt.Sprintf(
				`[wardhook] asked: inspection failed for %q`,
				cmd.Raw,
			)
		}
		if rank(dec) > rank(overall) {
			overall = dec
			overallReason = reason
		}
	}
	return overall, overallReason
}

func evalCommand(cfg *Config, toolName string, cmd parser.Command) (hook.Decision, string) {
	return evalCommandWithTrace(cfg, toolName, cmd, nil)
}

// evalCommandWithTrace is the shared evaluation engine used by both
// Evaluate (sink == nil) and EvaluateTrace. When sink is non-nil it
// records one TraceEntry per Rule whose Tool field matched, including
// those that did not match the MatchSpec, so the caller can render a
// full per-rule decision report.
func evalCommandWithTrace(
	cfg *Config,
	toolName string,
	cmd parser.Command,
	sink *[]TraceEntry,
) (hook.Decision, string) {
	dec := hook.DecisionAllow
	reason := ""
	for _, r := range cfg.Rules {
		if r.Tool != "*" && r.Tool != toolName {
			continue
		}
		excepted, skip := evalRule(r, cmd, sink)
		if skip {
			continue
		}
		if excepted {
			continue
		}
		if rank(r.Action) > rank(dec) {
			dec = r.Action
			reason = formatReason(r, cmd)
		}
	}
	return dec, reason
}

// evalRule evaluates a single Rule against cmd, optionally appending a
// TraceEntry to sink. It returns (excepted, skip): skip is true when
// the rule's MatchSpec did not match (caller should continue); excepted
// is true when the rule matched but was overridden by its Except clause.
func evalRule(r Rule, cmd parser.Command, sink *[]TraceEntry) (bool, bool) {
	if !matchSpec(&r.Match, cmd) {
		if sink != nil {
			*sink = append(*sink, TraceEntry{
				RuleName: r.Name,
				Tool:     r.Tool,
				Action:   r.Action,
				Outcome:  OutcomeNoMatch,
			})
		}
		return false, true
	}
	excepted := false
	exceptDetail := ""
	if r.Except != nil && matchSpec(r.Except, cmd) {
		excepted = true
		exceptDetail = formatExceptDetail(r.Except)
	}
	if sink != nil {
		outcome := OutcomeMatched
		if excepted {
			outcome = OutcomeExcepted
		}
		*sink = append(*sink, TraceEntry{
			RuleName:     r.Name,
			Tool:         r.Tool,
			Action:       r.Action,
			Outcome:      outcome,
			ExceptDetail: exceptDetail,
		})
	}
	return excepted, false
}

// Decision priority ranks used to aggregate winning rules.
// Higher rank beats lower: deny > ask > allow.
const (
	rankUnknown = 0
	rankAllow   = 1
	rankAsk     = 2
	rankDeny    = 3
)

func rank(d hook.Decision) int {
	switch d {
	case hook.DecisionDeny:
		return rankDeny
	case hook.DecisionAsk:
		return rankAsk
	case hook.DecisionAllow:
		return rankAllow
	}
	return rankUnknown
}

func formatReason(r Rule, cmd parser.Command) string {
	verb := "matched"
	switch r.Action {
	case hook.DecisionDeny:
		verb = "denied"
	case hook.DecisionAsk:
		verb = "asked"
	case hook.DecisionAllow:
		verb = "allowed"
	}
	custom := ""
	if r.Reason != "" {
		custom = ": " + r.Reason
	}
	raw := cmd.Raw
	if raw == "" {
		raw = cmd.Name
	}
	out := fmt.Sprintf(`[wardhook] %s by rule %q: %s%s`, verb, r.Name, raw, custom)
	if r.Message != "" {
		out += "\nHint: " + r.Message
	}
	return out
}
