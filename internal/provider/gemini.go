package provider

import (
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// GeminiProvider is the Google Gemini CLI Provider stub. ReadInvocation
// and WriteDecision panic until the real implementation lands.
type GeminiProvider struct{}

// Name returns "gemini".
func (GeminiProvider) Name() string { return "gemini" }

// ReadInvocation is unimplemented; runHook's panic recover degrades
// the response to "ask" so users see a clear error message.
func (GeminiProvider) ReadInvocation(_ io.Reader) (*Invocation, error) {
	panic("provider/gemini: ReadInvocation not implemented")
}

// WriteDecision is unimplemented; see ReadInvocation.
func (GeminiProvider) WriteDecision(_ io.Writer, _ hook.Decision, _ string) error {
	panic("provider/gemini: WriteDecision not implemented")
}
