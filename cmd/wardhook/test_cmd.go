package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

const (
	defaultTestTool  = "Bash"
	toolRead         = "Read"
	toolWrite        = "Write"
	toolEdit         = "Edit"
	toolNotebookEdit = "NotebookEdit"
	toolGlob         = "Glob"
	toolWebFetch     = "WebFetch"
	toolWebSearch    = "WebSearch"
)

// isSupportedTestTool reports whether name is a tool accepted by runTest.
// Grep is excluded because it requires two non-trivial fields (path +
// pattern) that the single-string positional argument cannot represent.
func isSupportedTestTool(name string) bool {
	switch name {
	case defaultTestTool, toolRead, toolWrite, toolEdit, toolNotebookEdit, toolGlob, toolWebFetch, toolWebSearch:
		return true
	}
	return false
}

// resolveTool picks the toolName to evaluate against, following the
// resolution rules in the design doc:
//   - if toolFlag is set, it wins (after support check),
//   - else if exactly one rule is selected and its Tool is a concrete
//     name (not "*"), that wins,
//   - else fall back to Bash.
//
// Grep and other unknown tool names are rejected.
func resolveTool(cfg *rule.Config, ruleNames []string, toolFlag string) (string, error) {
	if toolFlag != "" {
		if !isSupportedTestTool(toolFlag) {
			return "", fmt.Errorf("tool %q is not supported by wardhook test", toolFlag)
		}
		return toolFlag, nil
	}
	if len(ruleNames) == 1 {
		for _, r := range cfg.Rules {
			if r.Name != ruleNames[0] {
				continue
			}
			if r.Tool != "" && r.Tool != "*" {
				if !isSupportedTestTool(r.Tool) {
					return defaultTestTool, nil
				}
				return r.Tool, nil
			}
			break
		}
	}
	return defaultTestTool, nil
}

// toolInputKey maps a tool name to the single tool_input field that
// carries the positional <command> argument. Tools not handled here
// are not supported by buildToolInput.
func toolInputKey(name string) (string, bool) {
	switch name {
	case defaultTestTool:
		return "command", true
	case toolRead, toolWrite, toolEdit, toolNotebookEdit:
		return "file_path", true
	case toolGlob:
		return "pattern", true
	case toolWebFetch:
		return "url", true
	case toolWebSearch:
		return "query", true
	}
	return "", false
}

// buildToolInput synthesises a tool_input JSON payload for toolName by
// placing command in the canonical single field for that tool.
func buildToolInput(toolName, command string) (json.RawMessage, error) {
	key, ok := toolInputKey(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %q is not supported by wardhook test", toolName)
	}
	body, err := json.Marshal(map[string]string{key: command})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

type headerInfo struct {
	ConfigPath        string
	Tool              string
	SelectedRuleNames []string
	TotalRules        int
	InputCommand      string
}

// formatTrace writes the human-readable trace described in the design
// doc to w. The output is deterministic for a given (header, trace)
// input to support golden-string tests.
func formatTrace(w io.Writer, h headerInfo, t rule.Trace) {
	fmt.Fprintf(w, "config: %s\n", h.ConfigPath)
	fmt.Fprintf(w, "tool:   %s\n", h.Tool)
	fmt.Fprintf(w, "rules:  %s\n", formatRulesHeader(h))
	fmt.Fprintf(w, "input:  %s\n", h.InputCommand)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "parsed commands (%d):\n", len(t.Commands))
	for i, c := range t.Commands {
		fmt.Fprintf(w, "  [%d] %s\n", i, formatParsedCommand(c.Command))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "rule trace:")
	writeRuleTrace(w, t)
	fmt.Fprintln(w)
	if t.Final == hook.DecisionAsk && anyInspectionFailed(t) {
		fmt.Fprintf(w, "final: %s (forced by inspection_failed)\n", t.Final)
	} else {
		fmt.Fprintf(w, "final: %s\n", t.Final)
	}
	if t.Reason != "" {
		fmt.Fprintf(w, "reason: %s\n", t.Reason)
	}
}

func formatRulesHeader(h headerInfo) string {
	if len(h.SelectedRuleNames) == 0 {
		return "(all)"
	}
	return fmt.Sprintf("%s (%d of %d)",
		strings.Join(h.SelectedRuleNames, ", "),
		len(h.SelectedRuleNames),
		h.TotalRules,
	)
}

func formatParsedCommand(cmd parser.Command) string {
	parts := []string{fmt.Sprintf("name=%s", cmd.Name)}
	if len(cmd.Flags) > 0 {
		parts = append(parts, fmt.Sprintf("flags=[%s]", sortedFlags(cmd.Flags)))
	}
	if len(cmd.Args) > 0 {
		parts = append(parts, fmt.Sprintf("args=%v", cmd.Args))
	}
	if cmd.InspectionFailed {
		parts = append(parts, "inspection_failed=true")
	}
	parts = append(parts, fmt.Sprintf("raw=%q", cmd.Raw))
	return strings.Join(parts, " ")
}

func sortedFlags(fs map[string]struct{}) string {
	names := make([]string, 0, len(fs))
	for f := range fs {
		names = append(names, f)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

type ruleHeader struct {
	Name   string
	Tool   string
	Action hook.Decision
}

type ruleLine struct {
	Header ruleHeader
	Lines  []string
}

// writeRuleTrace prints the per-rule, per-command outcome block.
// Rules are grouped by name and listed in first-encountered order so
// the output is deterministic and mirrors the rules slice in the config.
func writeRuleTrace(w io.Writer, t rule.Trace) {
	var order []ruleHeader
	collected := map[string]*ruleLine{}
	for ci, c := range t.Commands {
		for _, rt := range c.Rules {
			hdr := ruleHeader{Name: rt.RuleName, Tool: rt.Tool, Action: rt.Action}
			entry, ok := collected[hdr.Name]
			if !ok {
				entry = &ruleLine{Header: hdr}
				collected[hdr.Name] = entry
				order = append(order, hdr)
			}
			entry.Lines = append(entry.Lines, formatOutcomeLine(ci, rt))
		}
	}
	if len(order) == 0 {
		fmt.Fprintln(w, "  (no rules matched)")
		return
	}
	for _, hdr := range order {
		entry := collected[hdr.Name]
		fmt.Fprintf(w, "  %s (tool=%s, action=%s)\n",
			hdr.Name, hdr.Tool, hdr.Action)
		for _, line := range entry.Lines {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
}

func formatOutcomeLine(cmdIdx int, rt rule.TraceEntry) string {
	switch rt.Outcome {
	case rule.OutcomeNoMatch:
		return fmt.Sprintf("[%d] no match", cmdIdx)
	case rule.OutcomeExcepted:
		return fmt.Sprintf("[%d] MATCH -> EXCEPT (%s) -> skip",
			cmdIdx, rt.ExceptDetail)
	case rule.OutcomeMatched:
		return fmt.Sprintf("[%d] MATCH -> %s", cmdIdx, rt.Action)
	}
	return fmt.Sprintf("[%d] unknown outcome", cmdIdx)
}

func anyInspectionFailed(t rule.Trace) bool {
	for _, c := range t.Commands {
		if c.InspectionFailed {
			return true
		}
	}
	return false
}

type repeatableStrings []string

func (r *repeatableStrings) String() string { return strings.Join(*r, ",") }
func (r *repeatableStrings) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func runTest(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet(testSubcommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to wardhook.yaml (searches standard locations if omitted)")
	var ruleNames repeatableStrings
	fs.Var(&ruleNames, "rule", "rule name to evaluate (repeatable)")
	toolFlag := fs.String("tool", "", "tool name to evaluate as (default Bash)")
	if err := fs.Parse(args); err != nil {
		return exitTestArgError
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr,
			"usage: wardhook test [--config PATH] [--rule NAME ...] [--tool TOOL] <command>")
		return exitTestArgError
	}
	commandStr := fs.Arg(0)

	resolved, found := resolveConfigPath(*configPath)
	if !found {
		fmt.Fprintln(stderr, "[wardhook] no config found in standard locations")
		return exitTestArgError
	}

	cfg, err := rule.Load(resolved)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] config error: %v\n", err)
		return exitTestArgError
	}

	if verr := validateRuleNames(cfg, ruleNames); verr != nil {
		fmt.Fprintf(stderr, "[wardhook] %v\n", verr)
		return exitTestArgError
	}

	toolName, err := resolveTool(cfg, ruleNames, *toolFlag)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] %v\n", err)
		return exitTestArgError
	}

	toolInput, err := buildToolInput(toolName, commandStr)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] %v\n", err)
		return exitTestArgError
	}

	pp := pickParser(toolName, cfg)
	cmds, err := pp.Parse(toolName, toolInput)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] parse error: %v\n", err)
		return exitTestParseError
	}

	cfgEval := cfg
	if len(ruleNames) > 0 {
		cfgEval = filterRules(cfg, ruleNames)
	}
	trace := rule.EvaluateTrace(cfgEval, toolName, cmds)
	formatTrace(stdout, headerInfo{
		ConfigPath:        resolved,
		Tool:              toolName,
		SelectedRuleNames: ruleNames,
		TotalRules:        len(cfg.Rules),
		InputCommand:      commandStr,
	}, trace)
	return 0
}

func validateRuleNames(cfg *rule.Config, names []string) error {
	if len(names) == 0 {
		return nil
	}
	known := map[string]struct{}{}
	for _, r := range cfg.Rules {
		known[r.Name] = struct{}{}
	}
	var unknown []string
	for _, n := range names {
		if _, ok := known[n]; !ok {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	return fmt.Errorf("unknown rule(s): %s", strings.Join(unknown, ", "))
}

// filterRules returns a shallow copy of cfg with cfg.Rules restricted to
// those whose Name is in names. cfg.Defaults is shared so that the
// parser still receives the full cli_specs / value-taking flag set.
func filterRules(cfg *rule.Config, names []string) *rule.Config {
	want := map[string]struct{}{}
	for _, n := range names {
		want[n] = struct{}{}
	}
	out := *cfg
	out.Rules = out.Rules[:0:0]
	for _, r := range cfg.Rules {
		if _, ok := want[r.Name]; ok {
			out.Rules = append(out.Rules, r)
		}
	}
	return &out
}
