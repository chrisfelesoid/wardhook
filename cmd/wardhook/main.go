// Command wardhook filters Claude Code tool invocations from the
// PreToolUse hook. See docs/superpowers/specs for the full design.
package main

import "os"

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, os.Args))
}
