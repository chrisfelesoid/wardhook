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

// copilotFileOpInput captures filePath in both spellings since VS Code's
// hooks.md does not pin down whether createFile/deleteFile use camelCase
// or snake_case. We try both and normalize to file_path (Claude vocab)
// so the existing PassthroughParser picks it up.
type copilotFileOpInput struct {
	FilePathCamel string `json:"filePath"`
	FilePathSnake string `json:"file_path"`
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
	normalized, nErr := normalizeCopilotToolInput(in.ToolName, in.ToolInput)
	if nErr != nil {
		return nil, nErr
	}
	return []*Invocation{{
		ToolName:  claudeTool,
		ToolInput: normalized,
		CWD:       in.CWD,
		Raw:       raw,
	}}, nil
}

// normalizeCopilotToolInput rewrites the tool_input payload from Copilot's
// shape to Claude's where they differ. createFile is the only single-call
// shape that needs rewriting; everything else (runTerminalCommand,
// deleteFile, pushToGitHub, unknown) passes through unchanged.
func normalizeCopilotToolInput(toolName string, in json.RawMessage) (json.RawMessage, error) {
	if toolName != "createFile" {
		return in, nil
	}
	var fp copilotFileOpInput
	if err := json.Unmarshal(in, &fp); err != nil {
		// Malformed tool_input: pass through. PassthroughParser will
		// fail to extract file_path and the rule engine will treat
		// the Write call as having no path constraint inputs.
		return in, nil //nolint:nilerr // intentional: swallow malformed input and let downstream parser surface it
	}
	path := fp.FilePathSnake
	if path == "" {
		path = fp.FilePathCamel
	}
	if path == "" {
		return in, nil
	}
	return json.Marshal(map[string]string{"file_path": path})
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
