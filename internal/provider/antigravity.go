package provider

import (
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

// Name returns "antigravity".
func (AntigravityProvider) Name() string { return "antigravity" }

// ReadInvocations is filled in by a later task. Stub returns nil to
// keep the interface check at compile time for cmd/wardhook wiring.
func (AntigravityProvider) ReadInvocations(_ io.Reader) ([]*Invocation, error) {
	return nil, nil
}

// WriteDecision is filled in by a later task.
func (AntigravityProvider) WriteDecision(_ io.Writer, _ hook.Decision, _ string) error {
	return nil
}
