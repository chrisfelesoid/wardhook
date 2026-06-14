package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// AntigravityProvider implements Provider for Google Antigravity's
// PreToolUse hook. Antigravity emits snake_case tool names
// ("run_command", "view_file", "edit_file", "write_file", ...) with
// PascalCase argument keys ("CommandLine", "FilePath", "Path"). The
// provider normalizes both to Claude vocabulary so existing
// wardhook.yaml rules apply unchanged. Unknown tools (list_dir,
// grep_search, MCP "server/tool", ...) pass through with their
// original name. The response is Antigravity's flat top-level
// {"decision": ..., "reason": ...} JSON, not Claude's hookSpecificOutput.
type AntigravityProvider struct{}

// antigravityDecision is the wire shape Antigravity expects on stdout.
// NOT Claude's hookSpecificOutput — Antigravity uses a flat top-level
// {"decision": ..., "reason": ...} object. Reason is omitted when empty.
type antigravityDecision struct {
	Decision hook.Decision `json:"decision"`
	Reason   string        `json:"reason,omitempty"`
}

// Name returns "antigravity".
func (AntigravityProvider) Name() string { return "antigravity" }

// ReadInvocations is filled in by a later task. Stub returns nil to
// keep the interface check at compile time for cmd/wardhook wiring.
func (AntigravityProvider) ReadInvocations(_ io.Reader) ([]*Invocation, error) {
	return nil, nil
}

// WriteDecision emits Antigravity's PreToolUse response JSON. The
// hook.Decision string values ("allow"/"deny"/"ask") match Antigravity's
// decision vocabulary verbatim, so no per-value mapping is needed.
func (AntigravityProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(antigravityDecision{
		Decision: dec,
		Reason:   reason,
	})
}
