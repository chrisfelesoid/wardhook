// Package rule defines the YAML rule schema and the evaluation engine
// that consumes parsed commands to produce a hook decision.
package rule

import (
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// Config is the top-level wardhook.yaml document.
type Config struct {
	Version  int      `yaml:"version"`
	Defaults Defaults `yaml:"defaults,omitempty"`
	Rules    []Rule   `yaml:"rules"`
}

// Defaults holds document-wide defaults shared by all rules.
type Defaults struct {
	CLISpecs          map[string]*clispec.CLISpec `yaml:"cli_specs,omitempty"`
	RecursiveMaxDepth int                         `yaml:"recursive_max_depth,omitempty"`
}

// Rule is a single match-and-decide entry.
type Rule struct {
	Name   string        `yaml:"name"`
	Tool   string        `yaml:"tool"`
	Match  MatchSpec     `yaml:"match"`
	Except *MatchSpec    `yaml:"except,omitempty"`
	Action hook.Decision `yaml:"action"`
	Reason string        `yaml:"reason,omitempty"`
}

// MatchSpec describes the predicates a parsed command must satisfy.
type MatchSpec struct {
	Command        string              `yaml:"command,omitempty"`
	FlagsAll       []string            `yaml:"flags_all,omitempty"`
	FlagsAny       []string            `yaml:"flags_any,omitempty"`
	FlagAliases    map[string][]string `yaml:"flag_aliases,omitempty"`
	FlagValues     []FlagValueMatch    `yaml:"flag_values,omitempty"`
	SubcommandsAll SubcommandPaths     `yaml:"subcommands_all,omitempty"`
	SubcommandsAny SubcommandPaths     `yaml:"subcommands_any,omitempty"`
	Glob           *GlobMatch          `yaml:"glob,omitempty"`
	Regex          *RegexMatch         `yaml:"regex,omitempty"`
}

// SubcommandPath is a sequence of verbs to match. The flat form
// [pr, create] is normalized at load time so each element becomes
// a single-verb SubcommandPath (equivalent to [[pr], [create]]).
type SubcommandPath []string

// SubcommandPaths is a set of SubcommandPath values. Its YAML
// representation accepts a union of the flat form ([pr, create])
// and the nested form ([[pr, create]]).
type SubcommandPaths []SubcommandPath

// UnmarshalYAML decodes either the flat form
//
//	subcommands_any: [pr, create]
//
// (each element is treated as a depth-1 path, equivalent to
//
//	subcommands_any: [[pr], [create]]
//
// ) or the nested form
//
//	subcommands_any: [[pr, create], [issue, list]]
//
// Mixing flat and nested entries in the same sequence is an error.
func (s *SubcommandPaths) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("subcommands_*: must be a sequence (got %s)", describeYAMLKind(node.Kind))
	}
	if len(node.Content) == 0 {
		*s = nil
		return nil
	}
	firstKind := node.Content[0].Kind
	out := make(SubcommandPaths, 0, len(node.Content))
	switch firstKind {
	case yaml.ScalarNode:
		for i, child := range node.Content {
			if child.Kind != yaml.ScalarNode {
				return fmt.Errorf("subcommands_*[%d]: mixed flat and nested form", i)
			}
			out = append(out, SubcommandPath{child.Value})
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if child.Kind != yaml.SequenceNode {
				return fmt.Errorf("subcommands_*[%d]: mixed flat and nested form", i)
			}
			path := make(SubcommandPath, 0, len(child.Content))
			for j, v := range child.Content {
				if v.Kind != yaml.ScalarNode {
					return fmt.Errorf("subcommands_*[%d][%d]: must be a string", i, j)
				}
				path = append(path, v.Value)
			}
			out = append(out, path)
		}
	case yaml.DocumentNode, yaml.MappingNode, yaml.AliasNode:
		return errors.New("subcommands_*[0]: must be a string or list of strings")
	default:
		return errors.New("subcommands_*[0]: must be a string or list of strings")
	}
	*s = out
	return nil
}

// describeYAMLKind returns a human-readable label for a yaml.Kind
// value, used in error messages instead of the raw numeric kind.
func describeYAMLKind(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.DocumentNode:
		return "document"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// FlagValueMatch declares that flag Name must be present in
// cmd.FlagValues and its captured values must satisfy at least one
// of Glob or Regex. If both are declared, they are AND'd.
type FlagValueMatch struct {
	Name  string      `yaml:"name"`
	Glob  *GlobMatch  `yaml:"glob,omitempty"`
	Regex *RegexMatch `yaml:"regex,omitempty"`
}

// GlobMatch declares how a set of glob patterns is quantified
// over a set of inputs (Command.Args for MatchSpec.Glob, or
// captured values for FlagValueMatch.Glob).
type GlobMatch struct {
	Mode     GlobMode `yaml:"mode"`
	Patterns []string `yaml:"patterns"`
}

// GlobMode is the quantifier applied across the input set.
type GlobMode string

// GlobMode values.
const (
	GlobModeAny GlobMode = "any" // ∃input ∈ inputs : ∃pat ∈ patterns : match(input, pat)
	GlobModeAll GlobMode = "all" // ∀input ∈ inputs : ∃pat ∈ patterns : match(input, pat)
)

// RegexMatch declares a quantified regex match against a set of
// inputs. Patterns are Go RE2 syntax — lookahead, lookbehind, and
// backreferences are not supported. Patterns are not auto-anchored;
// use ^/$ explicitly for full-string match.
type RegexMatch struct {
	Mode     GlobMode `yaml:"mode"`     // "any" | "all" (reuses GlobMode)
	Patterns []string `yaml:"patterns"` // RE2 source strings

	// compiled is populated by Validate() at load time and used by
	// matchRegex at evaluation time. Not serialized.
	compiled []*regexp.Regexp `yaml:"-"`
}

// Validate reports errors in this RegexMatch. On success it
// populates the internal compiled regex slice. The where argument
// is embedded in error messages.
func (r *RegexMatch) Validate(where string) error {
	switch r.Mode {
	case GlobModeAny, GlobModeAll:
	case "":
		return fmt.Errorf("%s.mode is required (any|all)", where)
	default:
		return fmt.Errorf(`%s.mode must be "any" or "all" (got %q)`, where, r.Mode)
	}
	if len(r.Patterns) == 0 {
		return fmt.Errorf("%s.patterns must list at least one pattern", where)
	}
	compiled := make([]*regexp.Regexp, len(r.Patterns))
	for i, p := range r.Patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("%s.patterns[%d] invalid: %w", where, i, err)
		}
		compiled[i] = re
	}
	r.compiled = compiled
	return nil
}
