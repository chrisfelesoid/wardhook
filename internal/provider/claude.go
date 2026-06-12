package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// ClaudeProvider is the reference Provider implementation for Claude Code.
// It thinly wraps the internal/hook package so the existing PreToolUse
// schema is reused without translation.
type ClaudeProvider struct{}

// Name returns "claude".
func (ClaudeProvider) Name() string { return "claude" }

// ReadInvocation decodes Claude Code's PreToolUse JSON from r and returns
// it as an Invocation. The original JSON is preserved in Invocation.Raw.
func (ClaudeProvider) ReadInvocation(r io.Reader) (*Invocation, error) {
	in, err := hook.ReadInput(r)
	if err != nil {
		return nil, err
	}
	raw, mErr := json.Marshal(in)
	if mErr != nil {
		// Re-marshalling a struct we just decoded should never fail; if
		// it somehow does, treat it as an internal error.
		return nil, mErr
	}
	return &Invocation{
		ToolName:  in.ToolName,
		ToolInput: in.ToolInput,
		CWD:       in.CWD,
		Raw:       raw,
	}, nil
}

// WriteDecision emits Claude Code's hookSpecificOutput JSON to w.
func (ClaudeProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	return hook.WriteOutput(w, dec, reason)
}
