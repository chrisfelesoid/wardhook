package provider

import (
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// CodexProvider is the OpenAI Codex CLI Provider stub. ReadInvocation
// and WriteDecision panic until the real implementation lands.
type CodexProvider struct{}

// Name returns "codex".
func (CodexProvider) Name() string { return "codex" }

// ReadInvocation is unimplemented; runHook's panic recover degrades
// the response to "ask" so users see a clear error message.
func (CodexProvider) ReadInvocation(_ io.Reader) (*Invocation, error) {
	panic("provider/codex: ReadInvocation not implemented")
}

// WriteDecision is unimplemented; see ReadInvocation.
func (CodexProvider) WriteDecision(_ io.Writer, _ hook.Decision, _ string) error {
	panic("provider/codex: WriteDecision not implemented")
}
