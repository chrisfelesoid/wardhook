package clispec_test

import (
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
)

func TestCLISpec_Validate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		spec    *clispec.CLISpec
		wantErr string // substring; empty means no error expected
	}{
		{
			name:    "empty spec",
			spec:    &clispec.CLISpec{},
			wantErr: "must declare at least one of value_taking_flags, recurse",
		},
		{
			name: "recurse empty",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{},
			},
			wantErr: "recurse: must set at least one of flags, terminator, subcommands",
		},
		{
			name: "negative skip",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{
					Subcommands: map[string]*clispec.SubcommandRecurse{
						"run": {Skip: -1},
					},
				},
			},
			wantErr: `subcommands["run"].skip must be >= 0`,
		},
		{
			name: "empty subcommand verb",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{
					Subcommands: map[string]*clispec.SubcommandRecurse{
						"": {Skip: 1},
					},
				},
			},
			wantErr: "subcommands: empty verb",
		},
		{
			name: "empty value_taking_flags entry",
			spec: &clispec.CLISpec{
				ValueTakingFlags: []string{""},
			},
			wantErr: `value_taking_flags[0]: must be non-empty without "-" prefix`,
		},
		{
			name: "dash-prefixed value_taking_flag",
			spec: &clispec.CLISpec{
				ValueTakingFlags: []string{"-name"},
			},
			wantErr: `value_taking_flags[0]: must be non-empty without "-" prefix`,
		},
		{
			name: "valid value_taking_flags only",
			spec: &clispec.CLISpec{
				ValueTakingFlags: []string{"name", "v"},
			},
		},
		{
			name: "valid recurse.flags only",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{
					Flags: []string{"c"},
				},
			},
		},
		{
			name: "valid recurse.terminator only",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{
					Terminator: true,
				},
			},
		},
		{
			name: "valid recurse.subcommands",
			spec: &clispec.CLISpec{
				Recurse: &clispec.RecurseSpec{
					Subcommands: map[string]*clispec.SubcommandRecurse{
						"run":  {Skip: 1},
						"exec": {Skip: 1},
					},
				},
			},
		},
		{
			name: "full docker-like spec",
			spec: &clispec.CLISpec{
				ValueTakingFlags: []string{"name", "volume", "v", "e"},
				Recurse: &clispec.RecurseSpec{
					Terminator: true,
					Subcommands: map[string]*clispec.SubcommandRecurse{
						"run":  {Skip: 1},
						"exec": {Skip: 1},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := c.spec.Validate("test-cli")
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
