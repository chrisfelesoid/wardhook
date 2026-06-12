package hook

import (
	"encoding/json"
	"io"
)

type Input struct {
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionAsk   Decision = "ask"
)

type Output struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

// HookSpecificOutput is the inner payload of Output for the PreToolUse hook event.
//
//nolint:revive // name fixed by Claude Code hook protocol (hookSpecificOutput)
type HookSpecificOutput struct {
	HookEventName            string   `json:"hookEventName"`
	PermissionDecision       Decision `json:"permissionDecision"`
	PermissionDecisionReason string   `json:"permissionDecisionReason,omitempty"`
}

func ReadInput(r io.Reader) (*Input, error) {
	dec := json.NewDecoder(r)
	in := &Input{}
	if err := dec.Decode(in); err != nil {
		return nil, err
	}
	return in, nil
}

func WriteOutput(w io.Writer, d Decision, reason string) error {
	out := Output{
		HookSpecificOutput: HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       d,
			PermissionDecisionReason: reason,
		},
	}
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}
