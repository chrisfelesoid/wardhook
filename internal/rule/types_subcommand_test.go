package rule_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func TestSubcommandPaths_UnmarshalYAML_FlatForm(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	if err := yaml.Unmarshal([]byte("[pr, create]"), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := rule.SubcommandPaths{{"pr"}, {"create"}}
	if !equalPaths(got, want) {
		t.Errorf("flat form: got %v, want %v", got, want)
	}
}

func TestSubcommandPaths_UnmarshalYAML_NestedForm(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	src := "[[pr, create], [issue, list]]"
	if err := yaml.Unmarshal([]byte(src), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := rule.SubcommandPaths{{"pr", "create"}, {"issue", "list"}}
	if !equalPaths(got, want) {
		t.Errorf("nested form: got %v, want %v", got, want)
	}
}

func TestSubcommandPaths_UnmarshalYAML_NestedDepthOne(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	if err := yaml.Unmarshal([]byte("[[pr], [create]]"), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := rule.SubcommandPaths{{"pr"}, {"create"}}
	if !equalPaths(got, want) {
		t.Errorf("nested depth-1: got %v, want %v", got, want)
	}
}

func TestSubcommandPaths_UnmarshalYAML_EmptySequence(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	if err := yaml.Unmarshal([]byte("[]"), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty: got %v, want zero-length", got)
	}
}

func TestSubcommandPaths_UnmarshalYAML_MixedFormError(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	err := yaml.Unmarshal([]byte("[pr, [issue, list]]"), &got)
	if err == nil {
		t.Fatal("expected error for mixed flat/nested form")
	}
	if !strings.Contains(err.Error(), "mixed") {
		t.Errorf("error should mention 'mixed': %v", err)
	}
}

func TestSubcommandPaths_UnmarshalYAML_NotASequenceError(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	err := yaml.Unmarshal([]byte("pr"), &got)
	if err == nil {
		t.Fatal("expected error for scalar input")
	}
	if !strings.Contains(err.Error(), "must be a sequence") {
		t.Errorf("error should mention 'must be a sequence': %v", err)
	}
}

func TestSubcommandPaths_UnmarshalYAML_MapElementError(t *testing.T) {
	t.Parallel()
	var got rule.SubcommandPaths
	err := yaml.Unmarshal([]byte("[{a: b}]"), &got)
	if err == nil {
		t.Fatal("expected error for map element")
	}
	if !strings.Contains(err.Error(), "must be a string or list of strings") {
		t.Errorf("error should mention element kind constraint: %v", err)
	}
}

func equalPaths(a, b rule.SubcommandPaths) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
