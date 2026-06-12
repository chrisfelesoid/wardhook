package parser

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
)

// extractRecursionTargets applies the RecurseSpec's strategies
// (flags, terminator, subcommands) to a CallExpr and returns the
// inner command strings to be re-parsed. valueTaking is used by the
// subcommand strategy to correctly partition flags from positionals.
//
// The second return value is true when the caller should set
// Command.InspectionFailed.
func extractRecursionTargets(
	call *syntax.CallExpr,
	spec *clispec.RecurseSpec,
	valueTaking map[string]struct{},
) ([]string, bool) {
	if spec == nil {
		return nil, false
	}
	words := callWords(call)
	var targets []string
	failed := false

	for _, key := range spec.Flags {
		v, found, ok := lookupFlag(words, key)
		if !found {
			continue
		}
		if !ok {
			failed = true
			continue
		}
		if v != "" {
			targets = append(targets, v)
		}
	}

	if spec.Terminator {
		v, found := terminatorTail(words)
		if found && v != "" {
			targets = append(targets, v)
		}
	}

	if len(spec.Subcommands) > 0 {
		target, subFailed := subcommandTail(words, valueTaking, spec.Subcommands)
		if subFailed {
			failed = true
		}
		if target != "" {
			targets = append(targets, target)
		}
	}

	return targets, failed
}

// subcommandTail applies the subcommand recurse strategy and returns
// the joined inner command (or "" if none) plus a failed flag set when
// the subcommand was matched but its skip positionals were truncated.
func subcommandTail(
	words []string,
	valueTaking map[string]struct{},
	subcommands map[string]*clispec.SubcommandRecurse,
) (string, bool) {
	start, found, ok := extractSubcommandTarget(words, valueTaking, subcommands)
	if !found {
		return "", false
	}
	if !ok {
		return "", true
	}
	tail := words[start:]
	// Strip a leading "--" so docker exec ct -- rm -rf /
	// produces "rm -rf /" rather than "-- rm -rf /".
	if len(tail) > 0 && tail[0] == "--" {
		tail = tail[1:]
	}
	return strings.Join(tail, " "), false
}

// lookupFlag dispatches by flag length and returns (value, found, ok).
func lookupFlag(words []string, key string) (string, bool, bool) {
	if len(key) == 1 {
		return shortFlagValue(words, key)
	}
	return longFlagValue(words, key)
}

func terminatorTail(words []string) (string, bool) {
	for i, w := range words {
		if w == "--" {
			tail := words[i+1:]
			return strings.Join(tail, " "), true
		}
	}
	return "", false
}

func shortFlagValue(words []string, flag string) (string, bool, bool) {
	target := "-" + flag
	for i, w := range words {
		if w == target {
			if i+1 < len(words) {
				v := words[i+1]
				return v, true, v != ""
			}
			return "", true, false
		}
	}
	return "", false, false
}

func longFlagValue(words []string, flag string) (string, bool, bool) {
	prefix := "--" + flag + "="
	target := "--" + flag
	for i, w := range words {
		if v, ok := strings.CutPrefix(w, prefix); ok {
			return v, true, v != ""
		}
		if w == target {
			if i+1 < len(words) {
				v := words[i+1]
				return v, true, v != ""
			}
			return "", true, false
		}
	}
	return "", false, false
}

// extractSubcommandTarget walks words token by token, classifying
// each as flag/value/positional based on valueTaking. When the first
// positional matches a subcommand verb, it skips spec.Skip more
// positionals and returns the start INDEX in words for the inner cmd.
//
// Returns (innerStart, found, ok):
//
//	innerStart: index into words[] where inner cmd begins
//	found:      a subcommand verb was matched
//	ok:         all skip positionals were available (not truncated)
//
// If words runs out before the skip is satisfied, returns
// (_, true, false) so the caller can mark Command.InspectionFailed.
func extractSubcommandTarget(
	words []string,
	valueTaking map[string]struct{},
	subcommands map[string]*clispec.SubcommandRecurse,
) (int, bool, bool) {
	if len(subcommands) == 0 {
		return 0, false, false
	}
	positionalsSeen := 0
	var matchedSpec *clispec.SubcommandRecurse
	for i := 1; i < len(words); i++ {
		w := words[i]
		if w == "--" {
			return handleTerminator(i, matchedSpec, positionalsSeen)
		}
		if isFlagToken(w) {
			advance, truncated := consumeFlag(words, i, valueTaking)
			if truncated {
				if matchedSpec != nil {
					return 0, true, false
				}
				return 0, false, false
			}
			i = advance
			continue
		}
		// positional token
		start, done, found, ok := handlePositional(
			words,
			i,
			w,
			valueTaking,
			subcommands,
			&matchedSpec,
			&positionalsSeen,
		)
		if done {
			return start, found, ok
		}
	}
	if matchedSpec != nil {
		return 0, true, false
	}
	return 0, false, false
}

// handleTerminator handles a "--" token encountered while scanning.
// If the subcommand skip count has already been satisfied, "--" marks
// the start of the inner command (returned as found+ok). If the verb
// was matched but the skip count is not yet satisfied, the "--" is
// treated as truncation (found, not ok) so the caller marks
// InspectionFailed and degrades to ask (fail-closed). Otherwise, the
// terminator strategy handles "--" separately, so this function
// bails out with not-found.
func handleTerminator(i int, matchedSpec *clispec.SubcommandRecurse, positionalsSeen int) (int, bool, bool) {
	if matchedSpec == nil {
		return 0, false, false
	}
	if positionalsSeen >= matchedSpec.Skip {
		return i, true, true
	}
	return 0, true, false
}

// handlePositional dispatches a positional token. If it terminates
// the scan (verb mismatch / inner-cmd resolved), done=true with the
// terminal (start, found, ok) tuple.
func handlePositional(
	words []string,
	i int,
	w string,
	valueTaking map[string]struct{},
	subcommands map[string]*clispec.SubcommandRecurse,
	matchedSpec **clispec.SubcommandRecurse,
	positionalsSeen *int,
) (int, bool, bool, bool) {
	if *matchedSpec == nil {
		spec, ok := subcommands[w]
		if !ok || spec == nil {
			return 0, true, false, false
		}
		*matchedSpec = spec
		*positionalsSeen = 0
		if spec.Skip == 0 {
			return findInnerStart(words, i+1, valueTaking), true, true, true
		}
		return 0, false, false, false
	}
	*positionalsSeen++
	if *positionalsSeen >= (*matchedSpec).Skip {
		return findInnerStart(words, i+1, valueTaking), true, true, true
	}
	return 0, false, false, false
}

// isFlagToken reports whether w looks like a flag (starts with "-"
// and has another character).
func isFlagToken(w string) bool {
	return strings.HasPrefix(w, "-") && len(w) > 1
}

// consumeFlag handles one flag token at index i. Returns the new
// loop index (caller does i++ via "continue"; this returns the index
// the caller should set i to before continue) and whether the flag's
// value was truncated (value-taking flag without next token).
func consumeFlag(words []string, i int, valueTaking map[string]struct{}) (int, bool) {
	w := words[i]
	name, _, hasEq := strings.Cut(w[1:], "=")
	name = strings.TrimPrefix(name, "-")
	if hasEq {
		return i, false
	}
	if _, isValueTaking := valueTaking[name]; !isValueTaking {
		return i, false
	}
	if i+1 >= len(words) {
		return i, true
	}
	return i + 1, false
}

// findInnerStart scans words from idx forward, skipping any flags
// and their values, and returns the index of the first positional
// (or len(words) if none remain).
func findInnerStart(words []string, idx int, valueTaking map[string]struct{}) int {
	for i := idx; i < len(words); i++ {
		w := words[i]
		if w == "--" {
			return i
		}
		if isFlagToken(w) {
			advance, _ := consumeFlag(words, i, valueTaking)
			i = advance
			continue
		}
		return i
	}
	return len(words)
}
