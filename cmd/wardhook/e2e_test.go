package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from " + here)
		}
		dir = parent
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	out := filepath.Join(t.TempDir(), "wardhook")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/wardhook")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, b)
	}
	return out
}

// runScenario executes the wardhook binary against one scenario directory
// and compares the decision to expected.json.
//
// Required files in the scenario directory:
//   - input.json    : the PreToolUse JSON sent to stdin
//   - rules.yaml    : wardhook configuration
//   - expected.json : the expected hookSpecificOutput
//
// Optional file:
//   - provider.txt  : a single word (e.g. "codex") selecting the wardhook
//     subcommand. If missing, the default (Claude) path is
//     used. The file is trimmed; an empty trim is treated as
//     "no subcommand". I/O errors other than file-not-found
//     fail the test loudly.
func runScenario(t *testing.T, bin, dir string) {
	t.Helper()
	inputPath := filepath.Join(dir, "input.json")
	rulesPath := filepath.Join(dir, "rules.yaml")
	expectedPath := filepath.Join(dir, "expected.json")
	providerPath := filepath.Join(dir, "provider.txt")

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected: %v", err)
	}

	args := []string{}
	b, pErr := os.ReadFile(providerPath)
	switch {
	case pErr == nil:
		if sub := strings.TrimSpace(string(b)); sub != "" {
			args = append(args, sub)
		}
	case errors.Is(pErr, os.ErrNotExist):
		// provider.txt is optional; absence selects the default Claude path.
	default:
		t.Fatalf("provider.txt: %v", pErr)
	}
	args = append(args, "--config", rulesPath)
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(string(input))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("run: %v\nstderr: %s", runErr, stderr.String())
	}

	var got, want map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("got JSON: %v\n%s", jsonErr, stdout.String())
	}
	if jsonErr := json.Unmarshal(expected, &want); jsonErr != nil {
		t.Fatalf("expected JSON: %v", jsonErr)
	}
	gotDec := extractDecision(got)
	wantDec := extractDecision(want)
	if !reflect.DeepEqual(gotDec, wantDec) {
		t.Errorf("decision mismatch:\n got:  %s\n want: %s\n full out: %s",
			gotDec, wantDec, stdout.String())
	}
}

func TestE2E_Scenarios(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	root := repoRoot(t)
	scenariosDir := filepath.Join(root, "testdata", "scenarios")

	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		t.Fatalf("read scenarios dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no scenarios found")
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runScenario(t, bin, filepath.Join(scenariosDir, name))
		})
	}
}

func extractDecision(o map[string]any) string {
	if hso, ok := o["hookSpecificOutput"].(map[string]any); ok {
		if dec, _ := hso["permissionDecision"].(string); dec != "" {
			return dec
		}
	}
	if dec, _ := o["permission"].(string); dec != "" {
		return dec
	}
	if dec, _ := o["decision"].(string); dec != "" {
		return dec
	}
	return ""
}
