package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// CodexProvider implements Provider for OpenAI Codex CLI's pre-tool-use
// hook. Codex emits Claude-style tool_name ("Bash", "Read", ...) and
// tool_input ({"command": "..."}, {"file_path": "..."}), so the rule
// engine sees the same vocabulary as Claude Code.
type CodexProvider struct{}

// codexInput captures only the fields wardhook needs from Codex's
// pre-tool-use.command.input schema. session_id, turn_id, model,
// permission_mode, tool_use_id, transcript_path, agent_id, agent_type
// are deliberately not unmarshalled.
type codexInput struct {
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// Name returns "codex".
func (CodexProvider) Name() string { return "codex" }

// ReadInvocation decodes Codex's pre-tool-use.command.input JSON from r.
// Unknown fields are tolerated for forward compatibility with future
// Codex schema additions.
func (CodexProvider) ReadInvocation(r io.Reader) (*Invocation, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var in codexInput
	if uErr := json.Unmarshal(raw, &in); uErr != nil {
		return nil, uErr
	}
	return &Invocation{
		ToolName:  in.ToolName,
		ToolInput: in.ToolInput,
		CWD:       in.CWD,
		Raw:       raw,
	}, nil
}

// WriteDecision emits a Codex pre-tool-use.command.output JSON. Codex's
// hookSpecificOutput is wire-compatible with Claude Code's, so the same
// internal/hook.WriteOutput is reused.
func (CodexProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	return hook.WriteOutput(w, dec, reason)
}
