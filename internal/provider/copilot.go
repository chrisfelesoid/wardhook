package provider

import (
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// CopilotProvider implements Provider for VS Code GitHub Copilot's
// PreToolUse hook. Copilot emits camelCase tool names
// ("runTerminalCommand", "editFiles", "createFile", "deleteFile",
// "pushToGitHub") which the provider normalizes to Claude's vocabulary
// where a direct mapping exists. The response uses Copilot's
// hookSpecificOutput format, which is wire-compatible with Claude Code's
// (and Codex's), so internal/hook.WriteOutput is reused.
type CopilotProvider struct{}

// Name returns "copilot".
func (CopilotProvider) Name() string { return "copilot" }

// WriteDecision emits Copilot's hookSpecificOutput JSON. The format
// matches Claude Code's, so hook.WriteOutput is reused.
func (CopilotProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	return hook.WriteOutput(w, dec, reason)
}

// ReadInvocations is added incrementally in subsequent tasks. Until then
// it returns a single empty Invocation so the skeleton compiles.
func (CopilotProvider) ReadInvocations(_ io.Reader) ([]*Invocation, error) {
	return []*Invocation{{}}, nil
}
