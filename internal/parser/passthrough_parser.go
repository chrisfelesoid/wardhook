package parser

import "encoding/json"

// PassthroughParser handles every non-Bash tool by extracting a fixed
// set of fields from tool_input and stuffing them into Command.Args.
// Inputs are stored verbatim (no path absolutization). Unknown tool
// names produce an empty Command so that only tool: "*" rules can react.
type PassthroughParser struct{}

// passField describes one field to lift out of tool_input. The order
// of fields in the returned slice defines Args ordering.
type passField struct {
	key string
}

const (
	keyFilePath = "file_path"
	keyPattern  = "pattern"
	keyPath     = "path"
	keyURL      = "url"
	keyQuery    = "query"
)

// fieldsFor returns the field list for a given tool name. Returning
// (nil, false) signals an unknown tool, which produces an empty
// Command from Parse.
func fieldsFor(toolName string) ([]passField, bool) {
	switch toolName {
	case "Read", "Write", "Edit", "NotebookEdit":
		return []passField{{key: keyFilePath}}, true
	case "Glob":
		return []passField{{key: keyPattern}}, true
	case "Grep":
		return []passField{{key: keyPath}, {key: keyPattern}}, true
	case "WebFetch":
		return []passField{{key: keyURL}}, true
	case "WebSearch":
		return []passField{{key: keyQuery}}, true
	default:
		return nil, false
	}
}

// Parse extracts the configured fields for toolName from toolInput and
// returns a single Command. Unknown tools yield an empty Command so
// that only tool: "*" rules can react.
func (p *PassthroughParser) Parse(
	toolName string,
	toolInput json.RawMessage,
) ([]Command, error) {
	fields, known := fieldsFor(toolName)
	if !known {
		return []Command{{}}, nil
	}
	var input map[string]any
	if len(toolInput) > 0 {
		if err := json.Unmarshal(toolInput, &input); err != nil {
			return nil, err
		}
	}
	cmd := Command{Raw: string(toolInput)}
	for _, f := range fields {
		v, present := input[f.key]
		if !present {
			continue
		}
		s, isString := v.(string)
		if !isString {
			continue
		}
		cmd.Args = append(cmd.Args, s)
	}
	return []Command{cmd}, nil
}
