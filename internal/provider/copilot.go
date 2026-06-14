package provider

import (
	"encoding/json"
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

// copilotInput captures the minimum fields wardhook needs from Copilot's
// PreToolUse schema. timestamp, session_id, hook_event_name,
// transcript_path, tool_use_id are deliberately not unmarshalled.
type copilotInput struct {
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// Name returns "copilot".
func (CopilotProvider) Name() string { return "copilot" }

// WriteDecision emits Copilot's hookSpecificOutput JSON. The format
// matches Claude Code's, so hook.WriteOutput is reused.
func (CopilotProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	return hook.WriteOutput(w, dec, reason)
}

// ReadInvocations decodes Copilot's PreToolUse JSON from r and returns
// one or more Invocations. tool_name is normalized to Claude vocabulary
// where a direct mapping exists; the others pass through unchanged so
// users can match them via tool: "<name>" or tool: "*" rules.
func (CopilotProvider) ReadInvocations(r io.Reader) ([]*Invocation, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var in copilotInput
	if uErr := json.Unmarshal(raw, &in); uErr != nil {
		return nil, uErr
	}
	claudeTool := normalizeCopilotToolName(in.ToolName)
	return []*Invocation{{
		ToolName:  claudeTool,
		ToolInput: in.ToolInput,
		CWD:       in.CWD,
		Raw:       raw,
	}}, nil
}

// normalizeCopilotToolName maps Copilot's camelCase vocabulary onto
// Claude's. Tools without a direct Claude equivalent (deleteFile,
// pushToGitHub, and any future additions) pass through unchanged so
// users can match them with tool: "<name>" or tool: "*" cross-tool rules.
func normalizeCopilotToolName(s string) string {
	switch s {
	case "runTerminalCommand":
		return "Bash"
	case "editFiles":
		return "Edit"
	case "createFile":
		return "Write"
	default:
		return s
	}
}
