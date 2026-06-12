package rule_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/rule"
)

func TestRegexMatch_Validate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		spec    *rule.RegexMatch
		wantErr string // substring; empty means no error
	}{
		{
			name:    "empty spec",
			spec:    &rule.RegexMatch{},
			wantErr: "mode is required",
		},
		{
			name:    "invalid mode",
			spec:    &rule.RegexMatch{Mode: "foo", Patterns: []string{"."}},
			wantErr: `must be "any" or "all"`,
		},
		{
			name:    "empty patterns",
			spec:    &rule.RegexMatch{Mode: rule.GlobModeAny, Patterns: nil},
			wantErr: "at least one pattern",
		},
		{
			name:    "invalid regex syntax",
			spec:    &rule.RegexMatch{Mode: rule.GlobModeAny, Patterns: []string{"[invalid"}},
			wantErr: "invalid:",
		},
		{
			name: "valid any with 1 pattern",
			spec: &rule.RegexMatch{Mode: rule.GlobModeAny, Patterns: []string{"^valid$"}},
		},
		{
			name: "valid all with multiple patterns",
			spec: &rule.RegexMatch{Mode: rule.GlobModeAll, Patterns: []string{"a", "b", "c"}},
		},
		{
			name: "anchored chmod-style patterns",
			spec: &rule.RegexMatch{
				Mode: rule.GlobModeAny,
				Patterns: []string{
					`^[0-7]?[0-9]?7[0-9]?7$`,
					`^[augo]*\+[rwx]*w[rwx]*$`,
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := c.spec.Validate("test")
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err: got %v, want substring %q", err, c.wantErr)
			}
		})
	}
}
