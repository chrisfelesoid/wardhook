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
	defaultConfigPath       = "./wardhook.yaml"
	debugEnvVar             = "WARDHOOK_DEBUG"
	minArgsForSubcommand    = 2
	exitValidateConfigError = 1
	exitValidateFlagError   = 2
	validateSubcommand      = "validate"
	checkSubcommand         = "check"
	claudeSubcommand        = "claude"
	codexSubcommand         = "codex"
	cursorSubcommand        = "cursor"
	geminiSubcommand        = "gemini"
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
		case cursorSubcommand:
			return runHook(provider.CursorProvider{}, stdin, stdout, stderr, args[2:])
		case geminiSubcommand:
			return runHook(provider.GeminiProvider{}, stdin, stdout, stderr, args[2:])
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
	// Fail-safe: any panic in the engine degrades to ask + log.
	// safeWriteAsk swallows a secondary panic from the provider so the
	// exit-0 contract holds even when the provider itself is broken
	// (e.g. the Codex/Gemini stubs panic on WriteDecision too).
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "[wardhook] panic: %v\n%s\n", r, debug.Stack())
			safeWriteAsk(p, stdout, fmt.Sprintf("[wardhook] internal panic: %v", r))
		}
	}()

	fs := flag.NewFlagSet("wardhook", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "path to wardhook.yaml")
	if err := fs.Parse(flags); err != nil {
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] flag error: %v", err))
		return 0
	}

	inv, err := p.ReadInvocation(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] input error: %v\n", err)
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] input error: %v", err))
		return 0
	}
	debugLogf(stderr, "provider=%s tool=%s cwd=%s", p.Name(), inv.ToolName, inv.CWD)

	cfg, err := rule.Load(*configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "[wardhook] no config at %s (allowing)\n", *configPath)
			writeAllow(p, stdout, "")
			return 0
		}
		fmt.Fprintf(stderr, "[wardhook] config error: %v\n", err)
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] config error: %v", err))
		return 0
	}
	debugLogf(stderr, "loaded %d rules from %s", len(cfg.Rules), *configPath)

	pp := pickParser(inv.ToolName, cfg)
	cmds, err := pp.Parse(inv.ToolName, inv.ToolInput)
	if err != nil {
		fmt.Fprintf(stderr, "[wardhook] parse error: %v\n", err)
		writeAsk(p, stdout, fmt.Sprintf("[wardhook] parse error: %v", err))
		return 0
	}
	debugLogf(stderr, "parsed %d command(s)", len(cmds))

	dec, reason := rule.Evaluate(cfg, inv.ToolName, cmds)
	debugLogf(stderr, "decision=%s reason=%s", dec, reason)
	if werr := p.WriteDecision(stdout, dec, reason); werr != nil {
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
	configPath := fs.String("config", defaultConfigPath, "path to wardhook.yaml")
	if err := fs.Parse(args); err != nil {
		return exitValidateFlagError
	}
	if _, err := rule.Load(*configPath); err != nil {
		fmt.Fprintf(stderr, "[wardhook] validate error: %v\n", err)
		return exitValidateConfigError
	}
	fmt.Fprintln(stdout, "OK")
	return 0
}
