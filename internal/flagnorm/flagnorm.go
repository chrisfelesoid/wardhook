// Package flagnorm normalizes a command's words into a flag set, a
// value map for value-taking flags, and a list of non-flag arguments.
// The first word (command name) is dropped from all outputs.
package flagnorm

import "strings"

// Aliases maps a canonical short flag to its alternative spellings
// (typically long-form names). For example: {"r": {"recursive"}}.
type Aliases map[string][]string

// Normalize splits a slice of shell words into:
//
//   - flags: canonical flag-name set (post-alias normalization)
//   - values: captured values per canonical flag name; multi-occurrence
//     of the same flag appends to the slice in source order
//   - args: positional arguments (and any words past "--")
//
// valueTaking declares which flag names consume a following value via
// space or attached form. The "=" form ("--name=v" or "-name=v") is
// always captured regardless of valueTaking membership because the
// "=" delimiter removes ambiguity.
//
// ok=false signals that a declared value-taking flag had no value
// available at the end of words. Callers (e.g. BashParser) should
// translate this into Command.InspectionFailed so the rule engine
// can degrade the decision to ask.
//
// The first element of words is treated as the command name and
// excluded from all outputs.
func Normalize(
	words []string,
	aliases Aliases,
	valueTaking map[string]struct{},
) (map[string]struct{}, map[string][]string, []string, bool) {
	flags := make(map[string]struct{})
	values := make(map[string][]string)
	var args []string
	ok := true

	if len(words) == 0 {
		return flags, values, args, ok
	}

	reverse := buildReverse(aliases)
	terminated := false

	for i := 1; i < len(words); i++ {
		w := words[i]
		if terminated {
			args = append(args, w)
			continue
		}
		if w == "--" {
			terminated = true
			continue
		}
		switch {
		case strings.HasPrefix(w, "--"):
			i = handleDoubleDash(w, words, i, reverse, valueTaking, flags, values, &ok)
		case strings.HasPrefix(w, "-") && len(w) > 1:
			i = handleSingleDash(w, words, i, reverse, valueTaking, flags, values, &ok)
		default:
			args = append(args, w)
		}
	}
	return flags, values, args, ok
}

// handleDoubleDash processes a "--..." token and returns the (possibly
// advanced) cursor index so Normalize can skip a consumed value token.
func handleDoubleDash(
	w string,
	words []string,
	i int,
	reverse map[string]string,
	valueTaking map[string]struct{},
	flags map[string]struct{},
	values map[string][]string,
	ok *bool,
) int {
	name := strings.TrimPrefix(w, "--")
	if k, v, found := strings.Cut(name, "="); found {
		canon := canonical(k, reverse)
		flags[canon] = struct{}{}
		values[canon] = append(values[canon], v)
		return i
	}
	canon := canonical(name, reverse)
	flags[canon] = struct{}{}
	if !flagTakesValue(canon, valueTaking) {
		return i
	}
	if i+1 >= len(words) {
		*ok = false
		return i
	}
	values[canon] = append(values[canon], words[i+1])
	return i + 1
}

// handleSingleDash processes a "-..." token. Returns the (possibly
// advanced) cursor index when a following word is consumed as value.
//
// The "=" form is always captured (length>=2 name is treated as long).
// Otherwise the priority is: full-name long-flag match > leftmost
// value-taking char in bundle > existing bundled short-flag expansion.
func handleSingleDash(
	w string,
	words []string,
	i int,
	reverse map[string]string,
	valueTaking map[string]struct{},
	flags map[string]struct{},
	values map[string][]string,
	ok *bool,
) int {
	name := w[1:] // strip leading '-'
	if k, v, found := strings.Cut(name, "="); found && k != "" {
		canon := canonical(k, reverse)
		flags[canon] = struct{}{}
		values[canon] = append(values[canon], v)
		return i
	}
	if len(name) == 1 {
		canon := canonical(name, reverse)
		flags[canon] = struct{}{}
		if !flagTakesValue(canon, valueTaking) {
			return i
		}
		if i+1 >= len(words) {
			*ok = false
			return i
		}
		values[canon] = append(values[canon], words[i+1])
		return i + 1
	}
	// length >= 2, no '='
	if flagTakesValue(canonical(name, reverse), valueTaking) {
		canon := canonical(name, reverse)
		flags[canon] = struct{}{}
		if i+1 >= len(words) {
			*ok = false
			return i
		}
		values[canon] = append(values[canon], words[i+1])
		return i + 1
	}
	// bundle expansion: left-to-right; stop at first value-taking char
	for idx, r := range name {
		c := string(r)
		canon := canonical(c, reverse)
		flags[canon] = struct{}{}
		if !flagTakesValue(canon, valueTaking) {
			continue
		}
		rest := name[idx+len(c):]
		if rest != "" {
			values[canon] = append(values[canon], rest)
			return i
		}
		if i+1 >= len(words) {
			*ok = false
			return i
		}
		values[canon] = append(values[canon], words[i+1])
		return i + 1
	}
	return i
}

func flagTakesValue(name string, valueTaking map[string]struct{}) bool {
	if len(valueTaking) == 0 {
		return false
	}
	_, ok := valueTaking[name]
	return ok
}

func buildReverse(a Aliases) map[string]string {
	r := make(map[string]string)
	for canon, alts := range a {
		for _, alt := range alts {
			r[alt] = canon
		}
	}
	return r
}

func canonical(name string, reverse map[string]string) string {
	if c, ok := reverse[name]; ok {
		return c
	}
	return name
}
