package rule

import (
	"fmt"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// Load reads and strict-parses a wardhook.yaml file at path.
// Unknown keys cause an error.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	cfg := &Config{}
	if derr := dec.Decode(cfg); derr != nil {
		return nil, fmt.Errorf("yaml: %w", derr)
	}
	if verr := validate(cfg); verr != nil {
		return nil, verr
	}
	return cfg, nil
}

// ResolvedValueTakingFlags walks all rules and returns the set of
// flag names that should be treated as value-taking by the parser,
// keyed by command name. The "" key holds names from rules with no
// command field (apply to any command). Alt names from
// match.flag_aliases are also included so the parser can consume the
// value regardless of which spelling appears in the actual command.
func (c *Config) ResolvedValueTakingFlags() map[string]map[string]struct{} {
	if c == nil || len(c.Rules) == 0 {
		return nil
	}
	out := map[string]map[string]struct{}{}
	for _, r := range c.Rules {
		if len(r.Match.FlagValues) == 0 {
			continue
		}
		cmd := r.Match.Command
		bucket, ok := out[cmd]
		if !ok {
			bucket = map[string]struct{}{}
			out[cmd] = bucket
		}
		for _, fv := range r.Match.FlagValues {
			bucket[fv.Name] = struct{}{}
			if alts, hit := r.Match.FlagAliases[fv.Name]; hit {
				for _, alt := range alts {
					bucket[alt] = struct{}{}
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validate(cfg *Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("version: must be 1, got %d", cfg.Version)
	}
	if err := validateDefaults(&cfg.Defaults); err != nil {
		return err
	}
	for i, r := range cfg.Rules {
		if r.Name == "" {
			return fmt.Errorf("rules[%d].name is required", i)
		}
		if r.Tool == "" {
			return fmt.Errorf("rules[%d].tool is required", i)
		}
		switch r.Action {
		case hook.DecisionAllow, hook.DecisionDeny, hook.DecisionAsk:
		default:
			return fmt.Errorf("rules[%d].action must be one of deny|ask|allow (got %q)", i, r.Action)
		}
		if err := validateMatchSpec(i, "match", &r.Match); err != nil {
			return err
		}
		if r.Except != nil {
			if err := validateMatchSpec(i, "except", r.Except); err != nil {
				return err
			}
		}
		if err := validateSubcommandsTool(i, &r); err != nil {
			return err
		}
		if err := validateSubcommandPaths(i, &r); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaults(d *Defaults) error {
	if d.RecursiveMaxDepth < 0 {
		return fmt.Errorf("defaults.recursive_max_depth must be >= 0, got %d",
			d.RecursiveMaxDepth)
	}
	for name, spec := range d.CLISpecs {
		if spec == nil {
			return fmt.Errorf("defaults.cli_specs[%q]: must not be null", name)
		}
		if err := spec.Validate(name); err != nil {
			return err
		}
	}
	return nil
}

func validateMatchSpec(ruleIdx int, where string, spec *MatchSpec) error {
	if spec.Glob != nil {
		if err := validateGlobMatch(spec.Glob,
			fmt.Sprintf("rules[%d].%s.glob", ruleIdx, where)); err != nil {
			return err
		}
	}
	if spec.Regex != nil {
		if err := spec.Regex.Validate(
			fmt.Sprintf("rules[%d].%s.regex", ruleIdx, where)); err != nil {
			return err
		}
	}
	return validateFlagValues(ruleIdx, where, spec.FlagValues, spec.FlagAliases)
}

func validateGlobMatch(g *GlobMatch, where string) error {
	switch g.Mode {
	case GlobModeAny, GlobModeAll:
	case "":
		return fmt.Errorf("%s.mode is required (any|all)", where)
	default:
		return fmt.Errorf(`%s.mode must be "any" or "all" (got %q)`, where, g.Mode)
	}
	if len(g.Patterns) == 0 {
		return fmt.Errorf("%s.patterns must list at least one pattern", where)
	}
	for i, pat := range g.Patterns {
		if _, err := doublestar.Match(pat, ""); err != nil {
			return fmt.Errorf("%s.patterns[%d] invalid: %w", where, i, err)
		}
	}
	return nil
}

func validateFlagValues(
	ruleIdx int,
	where string,
	fvs []FlagValueMatch,
	aliases map[string][]string,
) error {
	if len(fvs) == 0 {
		return nil
	}
	reverse := buildReverseAliases(aliases)
	seen := map[string]struct{}{}
	for i, fv := range fvs {
		if err := validateFlagValueEntry(ruleIdx, where, i, fv); err != nil {
			return err
		}
		canon := canonicalName(fv.Name, reverse)
		if _, dup := seen[canon]; dup {
			return fmt.Errorf(
				`rules[%d].%s.flag_values: duplicate name %q`,
				ruleIdx, where, canon)
		}
		seen[canon] = struct{}{}
	}
	return nil
}

// validateSubcommandsTool enforces that subcommands_any / subcommands_all
// (in either match or except) appear only on tool: Bash rules. Args[0]
// has no consistent "subcommand" meaning on Read/Write/Glob/* etc., so
// the schema rejects the combination at load time rather than silently
// matching against file paths or URLs.
func validateSubcommandsTool(ruleIdx int, r *Rule) error {
	hasSubcommand := len(r.Match.SubcommandsAny) > 0 ||
		len(r.Match.SubcommandsAll) > 0
	if r.Except != nil {
		hasSubcommand = hasSubcommand ||
			len(r.Except.SubcommandsAny) > 0 ||
			len(r.Except.SubcommandsAll) > 0
	}
	if !hasSubcommand {
		return nil
	}
	if r.Tool != "Bash" {
		return fmt.Errorf(
			"rules[%d] %q: subcommands_any/subcommands_all is only valid for tool: Bash (got tool: %q)",
			ruleIdx, r.Name, r.Tool)
	}
	return nil
}

// validateSubcommandPaths checks that every SubcommandPath in match/except
// is non-empty and contains no empty verb strings. Empty paths or empty
// verbs are configuration errors because they have no useful semantics
// and most likely indicate a typo in the YAML.
func validateSubcommandPaths(ruleIdx int, r *Rule) error {
	if err := checkPaths(ruleIdx, "match.subcommands_any", r.Match.SubcommandsAny); err != nil {
		return err
	}
	if err := checkPaths(ruleIdx, "match.subcommands_all", r.Match.SubcommandsAll); err != nil {
		return err
	}
	if r.Except != nil {
		if err := checkPaths(ruleIdx, "except.subcommands_any", r.Except.SubcommandsAny); err != nil {
			return err
		}
		if err := checkPaths(ruleIdx, "except.subcommands_all", r.Except.SubcommandsAll); err != nil {
			return err
		}
	}
	return nil
}

func checkPaths(ruleIdx int, where string, paths SubcommandPaths) error {
	for i, p := range paths {
		if len(p) == 0 {
			return fmt.Errorf("rules[%d].%s[%d]: empty verb path is not allowed", ruleIdx, where, i)
		}
		for j, v := range p {
			if v == "" {
				return fmt.Errorf("rules[%d].%s[%d][%d]: empty verb is not allowed", ruleIdx, where, i, j)
			}
		}
	}
	return nil
}

func validateFlagValueEntry(ruleIdx int, where string, i int, fv FlagValueMatch) error {
	if fv.Name == "" {
		return fmt.Errorf("rules[%d].%s.flag_values[%d].name must be non-empty",
			ruleIdx, where, i)
	}
	if strings.HasPrefix(fv.Name, "-") {
		return fmt.Errorf(
			`rules[%d].%s.flag_values[%d].name must not start with "-" (write %q not %q)`,
			ruleIdx, where, i, strings.TrimLeft(fv.Name, "-"), fv.Name)
	}
	if fv.Glob == nil && fv.Regex == nil {
		return fmt.Errorf(
			"rules[%d].%s.flag_values[%d]: must declare at least one of glob, regex",
			ruleIdx, where, i)
	}
	if fv.Glob != nil {
		if err := validateGlobMatch(fv.Glob,
			fmt.Sprintf("rules[%d].%s.flag_values[%d].glob", ruleIdx, where, i)); err != nil {
			return err
		}
	}
	if fv.Regex != nil {
		if err := fv.Regex.Validate(
			fmt.Sprintf("rules[%d].%s.flag_values[%d].regex", ruleIdx, where, i)); err != nil {
			return err
		}
	}
	return nil
}
