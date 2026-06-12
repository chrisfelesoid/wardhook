// Package clispec defines per-CLI parsing and recursion specs that
// guide the BashParser when handling wrapper commands such as
// docker, kubectl, bash -c, etc. Defining these types in a separate
// package lets both internal/rule and internal/parser depend on
// them without creating a circular import.
package clispec

import "fmt"

// CLISpec describes how to parse and recurse into invocations
// of a particular CLI command (e.g. docker, kubectl).
type CLISpec struct {
	// ValueTakingFlags lists flag names whose next token is a value
	// (not a positional). Short flags use 1-char names ("v"), long
	// flags use multi-char names ("volume").
	ValueTakingFlags []string `yaml:"value_taking_flags,omitempty"`

	// Recurse describes how to extract embedded inner commands.
	Recurse *RecurseSpec `yaml:"recurse,omitempty"`
}

// RecurseSpec declares the strategies for extracting an inner
// command. All listed strategies are applied; each may produce
// zero or more inner command strings.
type RecurseSpec struct {
	// Flags lists value-bearing flags whose value is itself a shell
	// command string. Short flag names like "c" match -c <value>;
	// long flag names like "command" match --command=<value> or
	// --command <value>.
	Flags []string `yaml:"flags,omitempty"`

	// Terminator, when true, treats words after the "--" sentinel
	// as a single inner command (space-joined).
	Terminator bool `yaml:"terminator,omitempty"`

	// Subcommands maps a subcommand verb to a positional-offset rule.
	// When the first positional argument matches a key here, the
	// parser skips Skip positional args, then treats the rest as
	// the inner command.
	Subcommands map[string]*SubcommandRecurse `yaml:"subcommands,omitempty"`
}

// SubcommandRecurse declares positional skip count for a subcommand.
type SubcommandRecurse struct {
	Skip int `yaml:"skip"`
}

// Validate reports errors in this CLISpec. The cliName argument is
// embedded in error messages so callers can identify which entry
// of a wider cli_specs map failed validation.
func (c *CLISpec) Validate(cliName string) error {
	if len(c.ValueTakingFlags) == 0 && c.Recurse == nil {
		return fmt.Errorf("cli_specs[%q]: must declare at least one of value_taking_flags, recurse", cliName)
	}
	for i, name := range c.ValueTakingFlags {
		if name == "" || name[0] == '-' {
			return fmt.Errorf(`cli_specs[%q].value_taking_flags[%d]: must be non-empty without "-" prefix`, cliName, i)
		}
	}
	if c.Recurse != nil {
		return c.Recurse.validate(cliName)
	}
	return nil
}

func (r *RecurseSpec) validate(cliName string) error {
	if len(r.Flags) == 0 && !r.Terminator && len(r.Subcommands) == 0 {
		return fmt.Errorf("cli_specs[%q].recurse: must set at least one of flags, terminator, subcommands", cliName)
	}
	for verb, sub := range r.Subcommands {
		if verb == "" {
			return fmt.Errorf("cli_specs[%q].recurse.subcommands: empty verb", cliName)
		}
		if sub == nil || sub.Skip < 0 {
			return fmt.Errorf("cli_specs[%q].recurse.subcommands[%q].skip must be >= 0", cliName, verb)
		}
	}
	return nil
}
