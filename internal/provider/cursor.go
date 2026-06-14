package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// CursorProvider implements Provider for Cursor IDE's preToolUse hook.
// Cursor emits its own wire format and tool name vocabulary: tool_name
// "Shell" is normalized to Claude's "Bash" so existing rules apply, and
// the response uses {permission, agent_message} instead of Claude's
// hookSpecificOutput.
type CursorProvider struct{}

// cursorInput captures only the fields wardhook needs from Cursor's
// preToolUse schema. Other fields (conversation_id, generation_id,
// model, hook_event_name, cursor_version, workspace_roots, user_email,
// transcript_path, tool_use_id, agent_message) are deliberately not
// unmarshalled.
type cursorInput struct {
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// cursorOutput is the wire format Cursor expects on stdout. permission
// is required; agent_message is shown to the model and is omitted when
// empty.
type cursorOutput struct {
	Permission   hook.Decision `json:"permission"`
	AgentMessage string        `json:"agent_message,omitempty"`
}

// Name returns "cursor".
func (CursorProvider) Name() string { return "cursor" }

// ReadInvocations decodes Cursor's preToolUse JSON from r and returns it
// as a single-element Invocation slice. Unknown fields are tolerated for
// forward compatibility with future Cursor schema additions. tool_name
// "Shell" is normalized to "Bash" so the rule engine sees a single,
// Claude-aligned vocabulary.
func (CursorProvider) ReadInvocations(r io.Reader) ([]*Invocation, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var in cursorInput
	if uErr := json.Unmarshal(raw, &in); uErr != nil {
		return nil, uErr
	}
	return []*Invocation{{
		ToolName:  normalizeCursorToolName(in.ToolName),
		ToolInput: in.ToolInput,
		CWD:       in.CWD,
		Raw:       raw,
	}}, nil
}

// WriteDecision emits Cursor's preToolUse response JSON. The reason
// string is delivered to the agent (LLM) so it can understand why the
// call was blocked.
func (CursorProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	return json.NewEncoder(w).Encode(cursorOutput{
		Permission:   dec,
		AgentMessage: reason,
	})
}

// normalizeCursorToolName maps Cursor's tool name vocabulary onto
// Claude's. Only "Shell" needs translation; Read/Write/Grep share names
// with Claude, and Delete/Task/MCP:* are Cursor-only names that pass
// through unchanged so users can match them by their original spelling.
func normalizeCursorToolName(s string) string {
	if s == "Shell" {
		return "Bash"
	}
	return s
}
