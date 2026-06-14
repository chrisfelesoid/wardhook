package provider

import (
	"encoding/json"
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

const (
	antigravityKeyFilePathSnake = "file_path"
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

// antigravityInput captures the minimum fields wardhook needs from
// Antigravity's PreToolUse schema. stepIdx, conversationId,
// transcriptPath, artifactDirectoryPath are deliberately not unmarshalled.
type antigravityInput struct {
	ToolCall       antigravityToolCall `json:"toolCall"`
	WorkspacePaths []string            `json:"workspacePaths"`
}

type antigravityToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// antigravityRunCommandArgs is the args shape for the run_command tool.
type antigravityRunCommandArgs struct {
	CommandLine string `json:"CommandLine"`
}

// antigravityFileArgs captures the path field for view_file / edit_file /
// write_file across three plausible key spellings. The public docs use
// PascalCase but do not pin down which of Path / FilePath wins, so we try
// snake_case file_path → PascalCase FilePath → PascalCase Path in that
// order. The first non-empty value is normalized to {"file_path": ...}.
type antigravityFileArgs struct {
	FilePathSnake string `json:"file_path"`
	FilePath      string `json:"FilePath"`
	Path          string `json:"Path"`
}

// antigravityDecision is the wire shape Antigravity expects on stdout.
// NOT Claude's hookSpecificOutput — Antigravity uses a flat top-level
// {"decision": ..., "reason": ...} object. Reason is omitted when empty.
type antigravityDecision struct {
	Decision hook.Decision `json:"decision"`
	Reason   string        `json:"reason,omitempty"`
}

// Name returns "antigravity".
func (AntigravityProvider) Name() string { return "antigravity" }

// ReadInvocations decodes Antigravity's PreToolUse JSON from r and
// returns it as a single-element Invocation slice. tool_name is
// normalized to Claude vocabulary where a direct mapping exists; the
// others pass through unchanged so users can match them via tool:
// "<name>" or tool: "*" rules. workspacePaths[0] is used as CWD; an
// empty or missing workspacePaths falls back to "".
func (AntigravityProvider) ReadInvocations(r io.Reader) ([]*Invocation, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var in antigravityInput
	if uErr := json.Unmarshal(raw, &in); uErr != nil {
		return nil, uErr
	}
	claudeTool := normalizeAntigravityToolName(in.ToolCall.Name)
	normalized, nErr := normalizeAntigravityToolArgs(in.ToolCall.Name, in.ToolCall.Args)
	if nErr != nil {
		return nil, nErr
	}
	cwd := ""
	if len(in.WorkspacePaths) > 0 {
		cwd = in.WorkspacePaths[0]
	}
	return []*Invocation{{
		ToolName:  claudeTool,
		ToolInput: normalized,
		CWD:       cwd,
		Raw:       raw,
	}}, nil
}

// WriteDecision emits Antigravity's PreToolUse response JSON.
func (AntigravityProvider) WriteDecision(w io.Writer, dec hook.Decision, reason string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(antigravityDecision{
		Decision: dec,
		Reason:   reason,
	})
}

// normalizeAntigravityToolName maps Antigravity's snake_case vocabulary
// onto Claude's. Tools without a direct Claude equivalent (list_dir,
// find_by_name, grep_search, generate_image, MCP "server/tool", and any
// future additions) pass through unchanged so users can match them with
// tool: "<name>" or tool: "*" cross-tool rules.
func normalizeAntigravityToolName(s string) string {
	switch s {
	case "run_command":
		return ToolBash
	case "view_file":
		return ToolRead
	case "edit_file":
		return ToolEdit
	case "write_file":
		return ToolWrite
	default:
		return s
	}
}

// normalizeAntigravityToolArgs rewrites the args payload from
// Antigravity's shape to Claude's where they differ. run_command
// extracts CommandLine into {"command": ...}; view_file / edit_file /
// write_file try file_path → FilePath → Path in priority order and
// normalize to {"file_path": ...}. Unknown tools pass through unchanged.
func normalizeAntigravityToolArgs(toolName string, in json.RawMessage) (json.RawMessage, error) {
	switch toolName {
	case "run_command":
		var rc antigravityRunCommandArgs
		if err := json.Unmarshal(in, &rc); err != nil {
			return in, nil //nolint:nilerr // intentional: swallow malformed input and let downstream parser surface it
		}
		if rc.CommandLine == "" {
			return in, nil
		}
		return json.Marshal(map[string]string{"command": rc.CommandLine})

	case "view_file", "edit_file", "write_file":
		var fa antigravityFileArgs
		if err := json.Unmarshal(in, &fa); err != nil {
			return in, nil //nolint:nilerr // intentional: malformed args pass through
		}
		path := fa.FilePathSnake
		if path == "" {
			path = fa.FilePath
		}
		if path == "" {
			path = fa.Path
		}
		if path == "" {
			return in, nil
		}
		return json.Marshal(map[string]string{antigravityKeyFilePathSnake: path})

	default:
		return in, nil
	}
}
