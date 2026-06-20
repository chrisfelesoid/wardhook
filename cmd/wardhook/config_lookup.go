package main

import "os"

// defaultConfigCandidates is the priority-ordered list of paths
// resolveConfigPath scans when --config is not given. CWD-relative.
// Highest priority first.
//
//nolint:gochecknoglobals,goconst // immutable priority list; literals are intentionally inline for readability
var defaultConfigCandidates = []string{
	"wardhook.yaml",
	"wardhook.yml",
	".wardhook.yaml",
	".wardhook.yml",
	".agents/wardhook.yaml",
	".agents/wardhook.yml",
	".agents/.wardhook.yaml",
	".agents/.wardhook.yml",
}

// resolveConfigPath decides which path the subcommand should hand to
// rule.Load.
//
//   - If userPath is non-empty, it is returned as-is (explicit --config
//     overrides search; the caller is responsible for handling a missing
//     file, mirroring the pre-existing behavior).
//   - If userPath is empty, the candidate list is scanned in priority
//     order. The first regular file (non-directory) is returned with
//     found=true. If every candidate misses (including parent-not-a-dir
//     cases), ("", false) is returned.
func resolveConfigPath(userPath string) (string, bool) {
	if userPath != "" {
		return userPath, true
	}
	for _, c := range defaultConfigCandidates {
		info, err := os.Stat(c)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return c, true
	}
	return "", false
}
