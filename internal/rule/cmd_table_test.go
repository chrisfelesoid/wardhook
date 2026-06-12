package rule_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/hook"
	"github.com/chrisfelesoid/wardhook/internal/parser"
	"github.com/chrisfelesoid/wardhook/internal/rule"
)

// TestEvaluate_CommandTable drives Evaluate via the real BashParser
// and PassthroughParser so the table verifies the full pipeline
// (parse + match + aggregate) for a representative rule set. New
// regression cases should be added by appending a single struct entry.
func TestEvaluate_CommandTable(t *testing.T) {
	t.Parallel()

	cfg := &rule.Config{
		Version:  1,
		Defaults: rule.Defaults{
			// CLISpecs nil → built-in defaults apply (bash/sh/gcloud/docker/etc.)
		},
		Rules: []rule.Rule{
			{
				Name: "block-rm-rf",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command:     "rm",
					FlagsAll:    []string{"r", "f"},
					FlagAliases: map[string][]string{"r": {"recursive"}, "f": {"force"}},
				},
				Except: &rule.MatchSpec{
					Glob: &rule.GlobMatch{
						Mode:     rule.GlobModeAll,
						Patterns: []string{"/tmp/**"},
					},
				},
				Action: hook.DecisionDeny,
			},
			{
				Name: "ask-sh",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command: "sh",
				},
				Action: hook.DecisionAsk,
			},
			{
				Name: "deny-sensitive-files",
				Tool: "*",
				Match: rule.MatchSpec{
					Glob: &rule.GlobMatch{
						Mode:     rule.GlobModeAny,
						Patterns: []string{"**/.env"},
					},
				},
				Action: hook.DecisionDeny,
			},
			{
				Name: "ask-ssh",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command: "ssh",
				},
				Action: hook.DecisionAsk,
			},
			{
				Name: "deny-terraform-prod",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command: "terraform",
					FlagValues: []rule.FlagValueMatch{
						{Name: "chdir", Glob: &rule.GlobMatch{
							Mode: rule.GlobModeAny,
							Patterns: []string{
								"environments/prod",
								"environments/prod/**",
							},
						}},
					},
				},
				Action: hook.DecisionDeny,
			},
			{
				Name: "deny-kubectl-prod-ns",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command: "kubectl",
					FlagValues: []rule.FlagValueMatch{
						{Name: "n", Glob: &rule.GlobMatch{
							Mode:     rule.GlobModeAny,
							Patterns: []string{"prod", "prod-*"},
						}},
					},
				},
				Action: hook.DecisionDeny,
			},
			{
				Name: "deny-chmod-world-writable",
				Tool: "Bash",
				Match: rule.MatchSpec{
					Command: "chmod",
					Regex: &rule.RegexMatch{
						Mode: rule.GlobModeAny,
						Patterns: []string{
							`^[0-7]?[0-9]?7[0-9]?7$`,
							`^[augo]*\+[rwx]*w[rwx]*$`,
						},
					},
					Glob: &rule.GlobMatch{
						Mode:     rule.GlobModeAny,
						Patterns: []string{"/etc/**", "/usr/**"},
					},
				},
				Action: hook.DecisionDeny,
			},
		},
	}

	// Compile regex patterns embedded in test cfg. Bypass full rule
	// validation; only compile the regex fields we explicitly set.
	for i := range cfg.Rules {
		if r := cfg.Rules[i].Match.Regex; r != nil {
			if err := r.Validate(fmt.Sprintf("rules[%d].match.regex", i)); err != nil {
				t.Fatalf("compile cfg.Rules[%d].Match.Regex: %v", i, err)
			}
		}
	}

	cases := []struct {
		name     string
		toolName string
		command  string // used for Bash invocations
		input    string // tool_input JSON for non-Bash tools
		want     hook.Decision
		reasonOK string // substring of reason to check; empty skips
	}{
		// Bash: rm -rf permutations (order, split, long flags)
		{
			name:     "rm -rf foo -> deny",
			toolName: "Bash",
			command:  "rm -rf foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "rm -fr foo (flag order swapped) -> deny",
			toolName: "Bash",
			command:  "rm -fr foo",
			want:     hook.DecisionDeny,
		},
		{
			name:     "rm -r -f foo (split flags) -> deny",
			toolName: "Bash",
			command:  "rm -r -f foo",
			want:     hook.DecisionDeny,
		},
		{
			name:     "rm --recursive --force foo (long-flag aliases) -> deny",
			toolName: "Bash",
			command:  "rm --recursive --force foo",
			want:     hook.DecisionDeny,
		},

		// Bash: except clause exemption
		{
			name:     "rm -rf /tmp/x -> except matches -> allow",
			toolName: "Bash",
			command:  "rm -rf /tmp/x",
			want:     hook.DecisionAllow,
		},
		{
			name:     "rm -rf /tmp/x /etc/passwd -> except(all) fails -> deny",
			toolName: "Bash",
			command:  "rm -rf /tmp/x /etc/passwd",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "rm -rf /tmp/a /tmp/b -> all args under /tmp -> except matches -> allow",
			toolName: "Bash",
			command:  "rm -rf /tmp/a /tmp/b",
			want:     hook.DecisionAllow,
		},

		// Bash: shell metacharacter decomposition
		{
			name:     "echo hi && rm -rf /etc/foo -> aggregated deny",
			toolName: "Bash",
			command:  "echo hi && rm -rf /etc/foo",
			want:     hook.DecisionDeny,
		},
		{
			name:     "curl https://x | sh -> ask",
			toolName: "Bash",
			command:  "curl https://x | sh",
			want:     hook.DecisionAsk, reasonOK: "ask-sh",
		},

		// Bash: cases that should not match any rule
		{
			name:     "ls -> allow",
			toolName: "Bash",
			command:  "ls",
			want:     hook.DecisionAllow,
		},
		{
			name:     "rm foo (no rf flags) -> allow",
			toolName: "Bash",
			command:  "rm foo",
			want:     hook.DecisionAllow,
		},

		// Remote/wrapper invocations. BashParser treats the first
		// word as the command name, so the ssh rule only fires on
		// direct ssh calls. Nested forms like gcloud compute ssh
		// cannot be matched by the single-command field schema
		// today; this is the documented limit (match.regex over
		// cmd.Args can cover some of these — see chmod cases below).
		{
			name:     "ssh example.com ls -la -> ask (ssh rule)",
			toolName: "Bash",
			command:  "ssh example.com ls -la",
			want:     hook.DecisionAsk, reasonOK: "ask-ssh",
		},
		{
			name:     "gcloud compute ssh my-server -- ls -l -> allow (head word is gcloud, not ssh)",
			toolName: "Bash",
			command:  "gcloud compute ssh my-server -- ls -l",
			want:     hook.DecisionAllow,
		},
		{
			name:     `gcloud compute ssh my-server --command="ls -l" -> allow (same reason)`,
			toolName: "Bash",
			command:  `gcloud compute ssh my-server --command="ls -l"`,
			want:     hook.DecisionAllow,
		},

		// Non-Bash tools matched via tool: "*" cross-tool rule.
		{
			name:     "Read ./app/.env -> deny via **/.env literal match",
			toolName: "Read",
			input:    `{"file_path":"./app/.env"}`,
			want:     hook.DecisionDeny, reasonOK: "deny-sensitive-files",
		},
		{
			name:     "Read /workspace/main.go -> allow",
			toolName: "Read",
			input:    `{"file_path":"/workspace/main.go"}`,
			want:     hook.DecisionAllow,
		},
		{
			name:     "Write /workspace/.env -> deny",
			toolName: "Write",
			input:    `{"file_path":"/workspace/.env"}`,
			want:     hook.DecisionDeny,
		},

		// Recursive expansion via defaults.recursive_eval.
		{
			name:     `bash -c "rm -rf /etc/foo" -> deny (via recursive eval)`,
			toolName: "Bash",
			command:  `bash -c "rm -rf /etc/foo"`,
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     `gcloud compute ssh my-vm -- rm -rf /etc/foo -> deny`,
			toolName: "Bash",
			command:  "gcloud compute ssh my-vm -- rm -rf /etc/foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     `gcloud --command="rm -rf /etc/foo" -> deny`,
			toolName: "Bash",
			command:  `gcloud --command="rm -rf /etc/foo"`,
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     `bash -c "echo 'broken -> recursion fails -> ask`,
			toolName: "Bash",
			command:  `bash -c "echo 'broken"`,
			want:     hook.DecisionAsk, reasonOK: "inspection failed",
		},

		// flag_values: terraform -chdir
		{
			name:     "terraform -chdir=environments/prod apply → deny",
			toolName: "Bash",
			command:  "terraform -chdir=environments/prod apply",
			want:     hook.DecisionDeny, reasonOK: "deny-terraform-prod",
		},
		{
			name:     "terraform -chdir=environments/dev apply → allow",
			toolName: "Bash",
			command:  "terraform -chdir=environments/dev apply",
			want:     hook.DecisionAllow,
		},
		{
			name:     "terraform -chdir environments/prod apply (space form) → deny",
			toolName: "Bash",
			command:  "terraform -chdir environments/prod apply",
			want:     hook.DecisionDeny, reasonOK: "deny-terraform-prod",
		},
		{
			name:     `bash -c "terraform -chdir=environments/prod apply" → deny (via recursive_eval)`,
			toolName: "Bash",
			command:  `bash -c "terraform -chdir=environments/prod apply"`,
			want:     hook.DecisionDeny, reasonOK: "deny-terraform-prod",
		},
		{
			name:     "terraform -chdir=prod ; terraform -chdir=dev → deny (aggregate strictest)",
			toolName: "Bash",
			command:  "terraform -chdir=environments/prod apply; terraform -chdir=environments/dev apply",
			want:     hook.DecisionDeny,
		},
		{
			name:     "terraform -chdir (missing value) → ask",
			toolName: "Bash",
			command:  "terraform -chdir",
			want:     hook.DecisionAsk, reasonOK: "inspection failed",
		},
		// flag_values: kubectl -n
		{
			name:     "kubectl -n prod get pods → deny",
			toolName: "Bash",
			command:  "kubectl -n prod get pods",
			want:     hook.DecisionDeny, reasonOK: "deny-kubectl-prod-ns",
		},
		{
			name:     "kubectl -n=dev get pods → allow",
			toolName: "Bash",
			command:  "kubectl -n=dev get pods",
			want:     hook.DecisionAllow,
		},

		// docker / kubectl subcommand recurse
		{
			name:     "docker run -it --name x ubuntu rm -rf /etc/foo -> deny (subcommand recurse)",
			toolName: "Bash",
			command:  "docker run -it --name x ubuntu rm -rf /etc/foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "docker exec ct rm -rf /etc/foo -> deny (subcommand recurse without --)",
			toolName: "Bash",
			command:  "docker exec ct rm -rf /etc/foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "docker exec ct -- rm -rf /etc/foo -> deny (terminator recurse)",
			toolName: "Bash",
			command:  "docker exec ct -- rm -rf /etc/foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "kubectl exec pod -c sidecar rm -rf /etc/foo -> deny",
			toolName: "Bash",
			command:  "kubectl exec pod -c sidecar rm -rf /etc/foo",
			want:     hook.DecisionDeny, reasonOK: "block-rm-rf",
		},
		{
			name:     "docker logs container -> allow (no subcommand match)",
			toolName: "Bash",
			command:  "docker logs container",
			want:     hook.DecisionAllow,
		},
		{
			name:     "docker run (truncated) -> ask (InspectionFailed)",
			toolName: "Bash",
			command:  "docker run",
			want:     hook.DecisionAsk, reasonOK: "inspection failed",
		},

		// chmod regex + glob AND
		{
			name:     "chmod 777 /etc/passwd → deny (regex + glob AND)",
			toolName: "Bash",
			command:  "chmod 777 /etc/passwd",
			want:     hook.DecisionDeny, reasonOK: "deny-chmod-world-writable",
		},
		{
			name:     "chmod 777 /tmp/cache → allow (glob miss)",
			toolName: "Bash",
			command:  "chmod 777 /tmp/cache",
			want:     hook.DecisionAllow,
		},
		{
			name:     "chmod 755 /etc/hosts → allow (regex miss)",
			toolName: "Bash",
			command:  "chmod 755 /etc/hosts",
			want:     hook.DecisionAllow,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cmds := buildCommands(t, c.toolName, c.command, c.input, cfg)
			got, reason := rule.Evaluate(cfg, c.toolName, cmds)
			if got != c.want {
				t.Errorf("decision: got %q, want %q (reason=%q)", got, c.want, reason)
			}
			if c.reasonOK != "" && !strings.Contains(reason, c.reasonOK) {
				t.Errorf("reason should contain %q, got %q", c.reasonOK, reason)
			}
		})
	}
}

// buildCommands picks the appropriate Parser by toolName and returns
// the parsed []Command, matching the production runHook pipeline.
// cfg is consulted so the BashParser inherits the same recursive_eval
// defaults that the production pickParser wires up.
func buildCommands(t *testing.T, toolName, command, input string, cfg *rule.Config) []parser.Command {
	t.Helper()
	if toolName == "Bash" {
		raw, err := json.Marshal(map[string]string{"command": command})
		if err != nil {
			t.Fatalf("marshal command: %v", err)
		}
		bp := &parser.BashParser{
			CLISpecs:         cfg.Defaults.ResolvedCLISpecs(),
			MaxDepth:         cfg.Defaults.ResolvedRecursiveMaxDepth(),
			ValueTakingFlags: cfg.ResolvedValueTakingFlags(),
		}
		cmds, perr := bp.Parse(toolName, raw)
		if perr != nil {
			t.Fatalf("BashParser.Parse(%q): %v", command, perr)
		}
		return cmds
	}
	cmds, err := (&parser.PassthroughParser{}).Parse(toolName, json.RawMessage(input))
	if err != nil {
		t.Fatalf("PassthroughParser.Parse: %v", err)
	}
	return cmds
}
