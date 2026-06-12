package clispec_test

import (
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/clispec"
)

func TestBuiltins_AllPresent(t *testing.T) {
	t.Parallel()
	got := clispec.Builtins()
	want := []string{"bash", "sh", "docker", "podman", "kubectl", "gcloud", "nsenter"}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("built-in %q missing", name)
		}
	}
}

func TestBuiltins_AllValid(t *testing.T) {
	t.Parallel()
	got := clispec.Builtins()
	for name, spec := range got {
		if err := spec.Validate(name); err != nil {
			t.Errorf("built-in %s invalid: %v", name, err)
		}
	}
}

func TestBuiltins_DockerSubcommands(t *testing.T) {
	t.Parallel()
	docker := clispec.Builtins()["docker"]
	if docker == nil || docker.Recurse == nil {
		t.Fatal("docker builtin missing or has no recurse")
	}
	if docker.Recurse.Subcommands["run"].Skip != 1 {
		t.Errorf("docker.run.skip: %d", docker.Recurse.Subcommands["run"].Skip)
	}
	if docker.Recurse.Subcommands["exec"].Skip != 1 {
		t.Errorf("docker.exec.skip: %d", docker.Recurse.Subcommands["exec"].Skip)
	}
	if !docker.Recurse.Terminator {
		t.Errorf("docker.terminator should be true")
	}
}

func TestBuiltins_ImmutableAcrossCalls(t *testing.T) {
	t.Parallel()
	a := clispec.Builtins()
	a["bash"].Recurse.Flags = nil // mutate the returned copy
	b := clispec.Builtins()
	if len(b["bash"].Recurse.Flags) == 0 {
		t.Error("Builtins() should return a fresh deep copy each call")
	}
}
