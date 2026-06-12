package clispec

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed builtins/*.yaml
var builtinsFS embed.FS

//nolint:gochecknoglobals // lazy-init cache for embedded built-in specs
var (
	builtinsOnce sync.Once
	builtinsMap  map[string]*CLISpec
	errBuiltins  error
)

// Builtins returns a deep copy of the built-in CLI specs for bash,
// sh, docker, podman, kubectl, gcloud, and nsenter. The map is
// loaded lazily on first call and cached. Callers receive a fresh
// deep copy each invocation; mutating the result does not affect
// subsequent callers.
func Builtins() map[string]*CLISpec {
	builtinsOnce.Do(loadBuiltins)
	if errBuiltins != nil {
		panic(fmt.Sprintf("clispec.Builtins: %v", errBuiltins))
	}
	out := make(map[string]*CLISpec, len(builtinsMap))
	for k, v := range builtinsMap {
		out[k] = cloneSpec(v)
	}
	return out
}

// LoadBuiltins forces eager loading of the embedded YAML files and
// returns any error encountered. Useful for surfacing parse errors
// at init time in tests.
func LoadBuiltins() error {
	builtinsOnce.Do(loadBuiltins)
	return errBuiltins
}

func loadBuiltins() {
	out := map[string]*CLISpec{}
	walkErr := fs.WalkDir(builtinsFS, "builtins", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		raw, readErr := builtinsFS.ReadFile(p)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", p, readErr)
		}
		spec := &CLISpec{}
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if decErr := dec.Decode(spec); decErr != nil {
			return fmt.Errorf("decode %s: %w", p, decErr)
		}
		name := strings.TrimSuffix(path.Base(p), ".yaml")
		if vErr := spec.Validate(name); vErr != nil {
			return fmt.Errorf("validate %s: %w", p, vErr)
		}
		out[name] = spec
		return nil
	})
	if walkErr != nil {
		errBuiltins = walkErr
		return
	}
	builtinsMap = out
}
