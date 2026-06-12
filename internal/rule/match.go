package rule

import (
	"regexp"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/chrisfelesoid/wardhook/internal/flagnorm"
	"github.com/chrisfelesoid/wardhook/internal/parser"
)

// matchSpec reports whether cmd satisfies every constraint in spec.
func matchSpec(spec *MatchSpec, cmd parser.Command) bool {
	if spec == nil {
		return false
	}
	if spec.Command != "" && spec.Command != cmd.Name {
		return false
	}
	canonical := canonicalizeFlags(cmd.Flags, spec.FlagAliases)
	if !matchFlagsAll(spec.FlagsAll, canonical) {
		return false
	}
	if !matchFlagsAny(spec.FlagsAny, canonical) {
		return false
	}
	if !evalGlobMatch(spec.Glob, cmd.Args) {
		return false
	}
	if !matchRegex(spec.Regex, cmd.Args) {
		return false
	}
	if !matchFlagValues(spec.FlagValues, cmd, spec.FlagAliases) {
		return false
	}
	return true
}

func matchFlagsAll(want []string, canonical map[string]struct{}) bool {
	for _, f := range want {
		if _, ok := canonical[f]; !ok {
			return false
		}
	}
	return true
}

func matchFlagsAny(want []string, canonical map[string]struct{}) bool {
	if len(want) == 0 {
		return true
	}
	for _, f := range want {
		if _, ok := canonical[f]; ok {
			return true
		}
	}
	return false
}

func canonicalizeFlags(flags map[string]struct{}, aliases flagnorm.Aliases) map[string]struct{} {
	if len(aliases) == 0 {
		return flags
	}
	reverse := buildReverseAliases(aliases)
	out := make(map[string]struct{}, len(flags))
	for f := range flags {
		if c, ok := reverse[f]; ok {
			out[c] = struct{}{}
		} else {
			out[f] = struct{}{}
		}
	}
	return out
}

// matchFlagValues evaluates the list of FlagValueMatch entries with
// AND semantics across entries. For each entry, the flag name (after
// canonicalization) must be present in cmd.FlagValues, and the
// captured values must satisfy at least one of entry.Glob and
// entry.Regex (or both, AND'd).
func matchFlagValues(spec []FlagValueMatch, cmd parser.Command, aliases flagnorm.Aliases) bool {
	if len(spec) == 0 {
		return true
	}
	reverse := buildReverseAliases(aliases)
	canonicalValues := canonicalizeFlagValues(cmd.FlagValues, reverse)
	for _, entry := range spec {
		name := canonicalName(entry.Name, reverse)
		vals, ok := canonicalValues[name]
		if !ok || len(vals) == 0 {
			return false
		}
		if entry.Glob != nil && !evalGlobMatch(entry.Glob, vals) {
			return false
		}
		if entry.Regex != nil && !matchRegex(entry.Regex, vals) {
			return false
		}
	}
	return true
}

func canonicalizeFlagValues(values map[string][]string, reverse map[string]string) map[string][]string {
	if len(values) == 0 {
		return values
	}
	if len(reverse) == 0 {
		return values
	}
	out := make(map[string][]string, len(values))
	for k, v := range values {
		canon := canonicalName(k, reverse)
		out[canon] = append(out[canon], v...)
	}
	return out
}

func buildReverseAliases(aliases map[string][]string) map[string]string {
	reverse := map[string]string{}
	for canon, alts := range aliases {
		for _, alt := range alts {
			reverse[alt] = canon
		}
	}
	return reverse
}

func canonicalName(name string, reverse map[string]string) string {
	if c, ok := reverse[name]; ok {
		return c
	}
	return name
}

// evalGlobMatch evaluates a GlobMatch against an input set. A nil
// spec is treated as passthrough (true). The empty-input case for
// mode=all returns false (vacuous-truth is not adopted: fail-closed).
func evalGlobMatch(g *GlobMatch, inputs []string) bool {
	if g == nil {
		return true
	}
	switch g.Mode {
	case GlobModeAny:
		return anyInputMatchesAnyPattern(inputs, g.Patterns)
	case GlobModeAll:
		if len(inputs) == 0 {
			return false
		}
		return allInputsMatchAnyPattern(inputs, g.Patterns)
	default:
		// Unknown mode (defensive). Load validation should prevent this.
		return false
	}
}

func anyInputMatchesAnyPattern(inputs, patterns []string) bool {
	for _, in := range inputs {
		for _, pat := range patterns {
			if ok, err := doublestar.Match(pat, in); err == nil && ok {
				return true
			}
		}
	}
	return false
}

func allInputsMatchAnyPattern(inputs, patterns []string) bool {
	for _, in := range inputs {
		matched := false
		for _, pat := range patterns {
			if ok, err := doublestar.Match(pat, in); err == nil && ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// matchRegex evaluates a RegexMatch against an input set. A nil spec
// is treated as passthrough (true). The empty-input case for mode=all
// returns false (vacuous-truth is not adopted: fail-closed). Patterns
// must have been compiled via Validate() before calling this; if
// compiled is nil the function returns false defensively.
func matchRegex(r *RegexMatch, inputs []string) bool {
	if r == nil {
		return true
	}
	if r.compiled == nil {
		// Defensive: Validate must populate compiled. If not, fail-closed.
		return false
	}
	switch r.Mode {
	case GlobModeAny:
		return anyInputMatchesAnyRegex(inputs, r.compiled)
	case GlobModeAll:
		if len(inputs) == 0 {
			return false
		}
		return allInputsMatchAnyRegex(inputs, r.compiled)
	default:
		return false
	}
}

func anyInputMatchesAnyRegex(inputs []string, compiled []*regexp.Regexp) bool {
	for _, in := range inputs {
		for _, re := range compiled {
			if re.MatchString(in) {
				return true
			}
		}
	}
	return false
}

func allInputsMatchAnyRegex(inputs []string, compiled []*regexp.Regexp) bool {
	for _, in := range inputs {
		matched := false
		for _, re := range compiled {
			if re.MatchString(in) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
