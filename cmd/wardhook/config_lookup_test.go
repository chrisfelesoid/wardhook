package main

import (
	"os"
	"path/filepath"
	"testing"
)

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_ExplicitPathIsReturnedAsIs(t *testing.T) {
	t.Chdir(t.TempDir())
	got, found := resolveConfigPath("explicit.yaml")
	if !found {
		t.Fatalf("found = false, want true for explicit path")
	}
	if got != "explicit.yaml" {
		t.Errorf("path = %q, want %q", got, "explicit.yaml")
	}
}

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_NoCandidates(t *testing.T) {
	t.Chdir(t.TempDir())
	got, found := resolveConfigPath("")
	if found {
		t.Errorf("found = true, want false (got path %q)", got)
	}
	if got != "" {
		t.Errorf("path = %q, want empty", got)
	}
}

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_Priority(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"wardhook.yaml", "wardhook.yaml"},
		{"wardhook.yml", "wardhook.yml"},
		{".wardhook.yaml", ".wardhook.yaml"},
		{".wardhook.yml", ".wardhook.yml"},
		{".agents/wardhook.yaml", ".agents/wardhook.yaml"},
		{".agents/wardhook.yml", ".agents/wardhook.yml"},
		{".agents/.wardhook.yaml", ".agents/.wardhook.yaml"},
		{".agents/.wardhook.yml", ".agents/.wardhook.yml"},
	}
	for _, tc := range cases {
		//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			full := filepath.Join(dir, tc.file)
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte("version: 1\nrules: []\n"), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, found := resolveConfigPath("")
			if !found {
				t.Fatalf("found = false, want true (file %s)", tc.file)
			}
			if got != tc.file {
				t.Errorf("path = %q, want %q", got, tc.file)
			}
		})
	}
}

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_HigherPriorityWinsOverLower(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "wardhook.yaml"), []byte("v"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", ".wardhook.yml"), []byte("v"), 0o600); err != nil {
		t.Fatalf("write yml: %v", err)
	}
	got, _ := resolveConfigPath("")
	if got != "wardhook.yaml" {
		t.Errorf("path = %q, want %q (highest priority should win)", got, "wardhook.yaml")
	}
}

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_DirectoryIsSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(filepath.Join(dir, "wardhook.yaml"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wardhook.yml"), []byte("v"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, found := resolveConfigPath("")
	if !found {
		t.Fatalf("found = false, want true")
	}
	if got != "wardhook.yml" {
		t.Errorf("path = %q, want %q (directory must be skipped)", got, "wardhook.yml")
	}
}

//nolint:paralleltest // t.Chdir mutates process state and forbids t.Parallel
func TestResolveConfigPath_ParentIsFileSkipsAgentsCandidates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".agents"), []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, found := resolveConfigPath("")
	if found {
		t.Errorf("found = true, want false (got %q; .agents as file should not yield matches)", got)
	}
}
