// Package provider abstracts the per-CLI (Claude Code, Codex, Gemini)
// hook input/output JSON schema. Rule evaluation is provider-agnostic.
package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// Invocation is the provider-neutral view of "a CLI is about to invoke a tool".
// All providers MUST normalize their fields into Claude vocabulary
// (ToolName "Bash" / "Read" / "Write" / "Edit" / ...) before constructing it.
type Invocation struct {
	// ToolName is normalized to Claude vocabulary.
	ToolName string

	// ToolInput is the structured tool input mapped to Claude-shaped fields
	// ({"command": "..."} for Bash, {"file_path": "..."} for Read, etc).
	ToolInput json.RawMessage

	// CWD is the working directory the tool will run in.
	CWD string

	// Raw is the original provider-specific JSON, unmodified. Kept for
	// diagnostics and debugging only; rule evaluation MUST NOT depend on it.
	Raw json.RawMessage
}

// Provider implements the I/O contract for one specific CLI.
// The rule engine never depends on Provider; only cmd/wardhook does.
type Provider interface {
	// Name returns "claude" / "codex" / "cursor" / "gemini" — used for logging and
	// subcommand routing. MUST match the subcommand string and be lowercase.
	Name() string

	// ReadInvocation reads one hook event from r and returns the
	// normalized Invocation. Returns an error if r is not parseable
	// as this provider's expected schema.
	ReadInvocation(r io.Reader) (*Invocation, error)

	// WriteDecision writes the decision back to w in this provider's
	// expected response format. The Decision values are the Claude
	// vocabulary ("allow" / "deny" / "ask"); each provider maps them
	// to its own action vocabulary internally.
	WriteDecision(w io.Writer, dec hook.Decision, reason string) error
}
