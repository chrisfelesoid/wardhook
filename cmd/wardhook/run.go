package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/provider"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

const (
	debugEnvVar             = "WARDHOOK_DEBUG"
	minArgsForSubcommand    = 2
	exitValidateConfigError = 1
	exitValidateFlagError   = 2
	validateSubcommand      = "validate"
	checkSubcommand         = "check"
	claudeSubcommand        = "claude"
	codexSubcommand         = "codex"
	copilotSubcommand       = "copilot"
	cursorSubcommand        = "cursor"
	antigravitySubcommand   = "antigravity"
	testSubcommand          = "test"
	exitTestArgError        = 2
	exitTestParseError      = 3
)

// run is the testable entry point. It returns the process exit code.
// In hook mode it is always 0: the decision is communicated through
// the JSON written to stdout.
func run(stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	if len(args) >= minArgsForSubcommand {
		switch args[1] {
		case checkSubcommand, claudeSubcommand:
			return runHook(provider.ClaudeProvider{}, stdin, stdout, stderr, args[2:])
		case codexSubcommand:
			return runHook(provider.CodexProvider{}, stdin, stdout, stderr, args[2:])
		case copilotSubcommand:
			return runHook(provider.CopilotProvider{}, stdin, stdout, stderr, args[2:])
		case cursorSubcommand:
			return runHook(provider.CursorProvider{}, stdin, stdout, stderr, args[2:])
		case antigravitySubcommand:
			return runHook(provider.AntigravityProvider{}, stdin, stdout, stderr, args[2:])
		case validateSubcommand:
			return runValidateWithIO(stdout, stderr, args[2:])
		case testSubcommand:
			return runTest(stdout, stderr, args[2:])
		}
	}
	// No subcommand: default to Claude Code for backward compatibility.
	return runHook(provider.ClaudeProvider{}, stdin, stdout, stderr, args[1:])
}

func runHook(p provider.Provider, stdin io.Reader, stdout, stderr io.Writer, flags []string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "[wardhook] panic: %v\n%s\n", r, debug.Stack())
			safeWriteAsk(p, stdout, fmt.Sprintf("[wardhook] internal panic: %v", r))
		}
	}()

	fs := flag.NewFlagSet("wardhook", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to wardhook.yaml (searches standard locations if omitted)")
	if err := fs.Parse(flags); err != nil {
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] flag error: %v", err))
		return 0
	}

	invs, err := p.ReadInvocations(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] input error: %v\n", err)
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] input error: %v", err))
		return 0
	}
	debugLogf(stderr, "provider=%s invocations=%d", p.Name(), len(invs))

	resolved, found := resolveConfigPath(*configPath)
	if !found {
		fmt.Fprintln(stderr, "[wardhook] no config found in standard locations (allowing)")
		writeAllow(p, stdout, "")
		return 0
	}
	debugLogf(stderr, "resolved config: %s", resolved)

	cfg, err := rule.Load(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "[wardhook] no config at %s (allowing)\n", resolved)
			writeAllow(p, stdout, "")
			return 0
		}
		fmt.Fprintf(stderr, "[wardhook] config error: %v\n", err)
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] config error: %v", err))
		return 0
	}
	debugLogf(stderr, "loaded %d rules from %s", len(cfg.Rules), resolved)

	finalDec := hook.DecisionAllow
	finalReason := ""
	for _, inv := range invs {
		debugLogf(stderr, "evaluating tool=%s cwd=%s", inv.ToolName, inv.CWD)
		pp := pickParser(inv.ToolName, cfg)
		cmds, perr := pp.Parse(inv.ToolName, inv.ToolInput)
		if perr != nil {
			fmt.Fprintf(stderr, "[wardhook] parse error: %v\n", perr)
			writeAsk(p, stdout, fmt.Sprintf("[wardhook] parse error: %v", perr))
			return 0
		}
		debugLogf(stderr, "parsed %d command(s)", len(cmds))
		dec, reason := rule.Evaluate(cfg, inv.ToolName, cmds)
		debugLogf(stderr, "decision=%s reason=%s", dec, reason)
		if stricter(dec, finalDec) {
			finalDec, finalReason = dec, reason
		}
	}

	if werr := p.WriteDecision(stdout, finalDec, finalReason); werr != nil {
		fmt.Fprintf(stderr, "[wardhook] output error: %v\n", werr)
	}
	return 0
}

func pickParser(toolName string, cfg *rule.Config) parser.Parser {
	if toolName == defaultTestTool {
		return &parser.BashParser{
			CLISpecs:         cfg.Defaults.ResolvedCLISpecs(),
			MaxDepth:         cfg.Defaults.ResolvedRecursiveMaxDepth(),
			ValueTakingFlags: cfg.ResolvedValueTakingFlags(),
		}
	}
	return &parser.PassthroughParser{}
}

func writeAllow(p provider.Provider, w io.Writer, reason string) {
	_ = p.WriteDecision(w, hook.DecisionAllow, reason)
}

func writeAsk(p provider.Provider, w io.Writer, reason string) {
	_ = p.WriteDecision(w, hook.DecisionAsk, reason)
}

// safeWriteAsk is the panic-safe variant used inside the runHook
// recover handler. If the provider's WriteDecision itself panics, we
// swallow it so the outer goroutine still returns exit code 0.
func safeWriteAsk(p provider.Provider, w io.Writer, reason string) {
	defer func() { _ = recover() }()
	_ = p.WriteDecision(w, hook.DecisionAsk, reason)
}

// stricter reports whether a is strictly stronger than b under deny > ask > allow.
// Used to aggregate decisions across multiple Invocations returned by a
// Provider (e.g. CopilotProvider expanding editFiles into N Edit invocations).
// The same ordering is applied inside rule.Evaluate when aggregating Commands.
func stricter(a, b hook.Decision) bool {
	return rank(a) > rank(b)
}

// Decision priority ranks used to aggregate decisions across Invocations.
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

func debugLogf(stderr io.Writer, format string, args ...any) {
	if os.Getenv(debugEnvVar) != "1" {
		return
	}
	fmt.Fprintf(stderr, "[wardhook:debug] "+format+"\n", args...)
}

// runValidateWithIO parses the config file and reports any structural
// errors. It writes "OK" to stdout on success. Unlike the hook mode,
// validate uses non-zero exit codes for errors so that CI pipelines
// can pick them up.
func runValidateWithIO(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet(validateSubcommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to wardhook.yaml (searches standard locations if omitted)")
	if err := fs.Parse(args); err != nil {
		return exitValidateFlagError
	}
	resolved, found := resolveConfigPath(*configPath)
	if !found {
		fmt.Fprintln(stderr, "[wardhook] no config found in standard locations")
		return exitValidateConfigError
	}
	if _, err := rule.Load(resolved); err != nil {
		fmt.Fprintf(stderr, "[wardhook] validate error: %v\n", err)
		return exitValidateConfigError
	}
	fmt.Fprintln(stdout, "OK")
	return 0
}
