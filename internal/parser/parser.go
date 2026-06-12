// Package parser turns a tool invocation (PreToolUse tool_input) into
// a flat slice of normalized Command values that rules can match against.
package parser

import "encoding/json"

// Command is a normalized view of a single command invocation.
// For Bash it represents one SimpleCommand; for non-Bash tools it
// represents the tool call itself with Name left empty.
type Command struct {
	Name       string              // "rm" etc. Empty for non-Bash tools.
	Flags      map[string]struct{} // canonical flag set
	FlagValues map[string][]string // captured values per canonical flag name (multi-occurrence preserves order)
	Args       []string            // positional arguments (paths/values)
	Raw        string              // original command text, used for reason messages

	// InspectionFailed marks a Command whose internal inspection
	// (recursive wrapper expansion, value-taking flag capture, ...)
	// could not be completed. rule.Evaluate degrades such commands
	// to ask so that uninspectable tokens do not silently slip
	// through with allow.
	InspectionFailed bool
}

// Parser converts a tool invocation into one or more Commands.
type Parser interface {
	Parse(toolName string, toolInput json.RawMessage) ([]Command, error)
}
