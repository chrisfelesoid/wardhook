package parser_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/parser"
)

func TestPassthroughParser_ReadFilePath(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("Read", json.RawMessage(`{"file_path": "./src/main.go"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("want 1 cmd, got %d", len(cmds))
	}
	if cmds[0].Name != "" {
		t.Errorf("Name: got %q, want empty", cmds[0].Name)
	}
	// Path is stored verbatim, no absolutization.
	if !reflect.DeepEqual(cmds[0].Args, []string{"./src/main.go"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestPassthroughParser_GrepPathAndPattern(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("Grep", json.RawMessage(`{"path": "/etc", "pattern": "root"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("want 1 cmd, got %d", len(cmds))
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"/etc", "root"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestPassthroughParser_WebFetchURL(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("WebFetch", json.RawMessage(`{"url": "https://example.com"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"https://example.com"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestPassthroughParser_UnknownTool(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("MysteryTool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("want 1 cmd, got %d", len(cmds))
	}
	if cmds[0].Name != "" || len(cmds[0].Args) != 0 {
		t.Errorf("unknown tool should produce empty command, got %+v", cmds[0])
	}
}

func TestPassthroughParser_GlobPattern(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("Glob", json.RawMessage(`{"pattern": "**/*.go"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"**/*.go"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestPassthroughParser_WebSearchQuery(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("WebSearch", json.RawMessage(`{"query": "go testing"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(cmds[0].Args, []string{"go testing"}) {
		t.Errorf("Args: %v", cmds[0].Args)
	}
}

func TestPassthroughParser_MissingField(t *testing.T) {
	t.Parallel()
	p := &parser.PassthroughParser{}
	cmds, err := p.Parse("Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Parse should not error, got: %v", err)
	}
	if len(cmds) != 1 || len(cmds[0].Args) != 0 {
		t.Errorf("missing field: got %+v", cmds)
	}
}
