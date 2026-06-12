package parser

import (
	"bytes"
	"encoding/json"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
	"github.com/chrisfelesoid/wardhook/internal/flagnorm"
)

// BashParser parses a Bash command string from tool_input.command,
// walks the AST, and emits one Command per CallExpr it discovers
// (pipes, ;, &&, ||, $(...), `...`).
//
// CLISpecs defines per-CLI value-taking flags and recursion strategies
// (flags / terminator / subcommands). When non-nil, BashParser
// expands wrapper commands accordingly up to MaxDepth levels deep.
//
// ValueTakingFlags is a per-rule additive set (populated from
// match.flag_values declarations). It is unioned with the
// per-CLI ValueTakingFlags from CLISpecs at parse time.
type BashParser struct {
	CLISpecs         map[string]*clispec.CLISpec
	MaxDepth         int
	ValueTakingFlags map[string]map[string]struct{}

	// depth is the current recursion depth, propagated through copies.
	depth int
}

func (b *BashParser) Parse(toolName string, toolInput json.RawMessage) ([]Command, error) {
	_ = toolName
	var ti struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(toolInput, &ti); err != nil {
		return nil, err
	}
	return b.parseCommand(ti.Command)
}

// parseCommand parses a raw Bash command string and emits Commands.
// It is shared between the top-level Parse entry point (which unmarshals
// tool_input.command first) and the recursive expansion path (which
// already has the inner command string in hand).
func (b *BashParser) parseCommand(command string) ([]Command, error) {
	if strings.TrimSpace(command) == "" {
		return nil, nil
	}

	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(command), "command")
	if err != nil {
		return nil, err
	}

	var cmds []Command
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		words := callWords(call)
		if len(words) == 0 {
			return true
		}
		valueSet := b.resolveValueTaking(words[0])
		flags, vals, args, normOK := flagnorm.Normalize(words, nil, valueSet)
		cmd := Command{
			Name:       words[0],
			Flags:      flags,
			FlagValues: vals,
			Args:       args,
			Raw:        printNode(call),
		}
		if !normOK {
			cmd.InspectionFailed = true
		}
		spec := b.specFor(cmd.Name)
		if spec != nil && spec.Recurse != nil {
			children, failed := b.expandRecursive(call, spec.Recurse, valueSet)
			if failed {
				cmd.InspectionFailed = true
			}
			cmds = append(cmds, cmd)
			cmds = append(cmds, children...)
			return true
		}
		cmds = append(cmds, cmd)
		return true
	})
	return cmds, nil
}

func (b *BashParser) specFor(name string) *clispec.CLISpec {
	if b.CLISpecs == nil {
		return nil
	}
	return b.CLISpecs[name]
}

// resolveValueTaking merges the per-CLI CLISpec set, the per-command
// rule set, and the "" wildcard rule set so the parser can correctly
// partition flags from positionals regardless of source.
//
// Recurse.Flags are folded in as value-taking too: by definition they
// carry the inner command as a value (e.g. bash -c "<inner>"). Without
// this fold the captured value would leak into wrapper.Args, where
// tool: "*" glob or regex rules would double-match it.
func (b *BashParser) resolveValueTaking(name string) map[string]struct{} {
	out := map[string]struct{}{}
	b.mergeCLISpecValueTaking(name, out)
	if per, ok := b.ValueTakingFlags[name]; ok {
		for f := range per {
			out[f] = struct{}{}
		}
	}
	if wild, ok := b.ValueTakingFlags[""]; ok {
		for f := range wild {
			out[f] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeCLISpecValueTaking pulls the CLISpec's value-taking flag names
// and recurse flag names into out.
func (b *BashParser) mergeCLISpecValueTaking(name string, out map[string]struct{}) {
	if b.CLISpecs == nil {
		return
	}
	spec, ok := b.CLISpecs[name]
	if !ok || spec == nil {
		return
	}
	for _, f := range spec.ValueTakingFlags {
		out[f] = struct{}{}
	}
	if spec.Recurse != nil {
		for _, f := range spec.Recurse.Flags {
			out[f] = struct{}{}
		}
	}
}

// expandRecursive extracts target strings via extractRecursionTargets
// and re-parses each with a depth-incremented BashParser. Failures or
// depth overruns are reflected in the returned failed flag.
func (b *BashParser) expandRecursive(
	call *syntax.CallExpr,
	spec *clispec.RecurseSpec,
	valueTaking map[string]struct{},
) ([]Command, bool) {
	targets, failed := extractRecursionTargets(call, spec, valueTaking)
	if len(targets) == 0 {
		return nil, failed
	}
	maxDepth := b.MaxDepth
	if maxDepth <= 0 {
		// Defensive default. rule.Defaults.ResolvedRecursiveMaxDepth
		// normally feeds this, but a directly-constructed BashParser
		// (e.g. in tests) may have left MaxDepth at zero.
		maxDepth = 3
	}
	if b.depth+1 > maxDepth {
		return nil, true
	}
	child := *b
	child.depth = b.depth + 1
	var collected []Command
	for _, t := range targets {
		t = unescapeDoubleQuoted(t)
		if t == "" {
			continue
		}
		sub, subErr := child.parseCommand(t)
		if subErr != nil {
			failed = true
			continue
		}
		collected = append(collected, sub...)
	}
	return collected, failed
}

func callWords(call *syntax.CallExpr) []string {
	words := make([]string, 0, len(call.Args))
	for _, w := range call.Args {
		words = append(words, wordLiteral(w))
	}
	return words
}

// wordLiteral renders a syntax.Word back to a best-effort literal
// string suitable for flagnorm. For unquoted literals this is the
// raw text; for quoted parts it concatenates the inner literals.
func wordLiteral(w *syntax.Word) string {
	var b strings.Builder
	for _, p := range w.Parts {
		switch part := p.(type) {
		case *syntax.Lit:
			b.WriteString(part.Value)
		case *syntax.SglQuoted:
			b.WriteString(part.Value)
		case *syntax.DblQuoted:
			for _, inner := range part.Parts {
				if lit, ok := inner.(*syntax.Lit); ok {
					b.WriteString(lit.Value)
					continue
				}
				b.WriteString(printNode(inner))
			}
		default:
			b.WriteString(printNode(p))
		}
	}
	return b.String()
}

// unescapeDoubleQuoted strips the backslash from the standard set of
// escapes recognized inside Bash double-quoted strings: \", \\, \$, \`,
// and \<newline>. mvdan.cc/sh/v3 preserves these escapes literally in
// Lit values inside DblQuoted parts, so the recursive evaluator has to
// undo them before re-parsing the inner command.
func unescapeDoubleQuoted(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '"', '\\', '$', '`', '\n':
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func printNode(n syntax.Node) string {
	var buf bytes.Buffer
	if err := syntax.NewPrinter().Print(&buf, n); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
