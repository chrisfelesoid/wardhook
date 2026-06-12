package rule

import "github.com/chrisfelesoid/wardhook/internal/parser"

// MatchSpecFn exposes the unexported matchSpec to the external rule_test
// package. Defined in an _test.go file so it stays out of the public API.
func MatchSpecFn(spec *MatchSpec, cmd parser.Command) bool {
	return matchSpec(spec, cmd)
}

// EvalGlobMatchFn exposes the unexported evalGlobMatch helper to the
// external rule_test package.
func EvalGlobMatchFn(spec *GlobMatch, inputs []string) bool {
	return evalGlobMatch(spec, inputs)
}

// MatchRegexFn exposes the unexported matchRegex helper to the
// external rule_test package.
func MatchRegexFn(spec *RegexMatch, inputs []string) bool {
	return matchRegex(spec, inputs)
}

// FormatExceptDetailForTest exposes formatExceptDetail for unit tests
// in package rule_test.
func FormatExceptDetailForTest(spec *MatchSpec) string {
	return formatExceptDetail(spec)
}
