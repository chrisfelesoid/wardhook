package rule

import "github.com/chrisfelesoid/wardhook/internal/clispec"

const defaultRecursiveMaxDepth = 3

// ResolvedCLISpecs returns the deep-merged CLISpecs: built-in entries
// plus any user-declared additions/overrides. The result is a fresh
// map; mutating it does not affect subsequent callers.
func (d Defaults) ResolvedCLISpecs() map[string]*clispec.CLISpec {
	if len(d.CLISpecs) == 0 {
		return clispec.Builtins()
	}
	return clispec.MergeCLISpecs(clispec.Builtins(), d.CLISpecs)
}

// ResolvedRecursiveMaxDepth returns the configured depth or the
// built-in default (3) when unset (zero value).
func (d Defaults) ResolvedRecursiveMaxDepth() int {
	if d.RecursiveMaxDepth > 0 {
		return d.RecursiveMaxDepth
	}
	return defaultRecursiveMaxDepth
}
