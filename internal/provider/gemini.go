package provider

import (
	"io"

	"github.com/chrisfelesoid/wardhook/internal/hook"
)

// GeminiProvider is the Google Gemini CLI Provider stub. ReadInvocations
// and WriteDecision panic until the real implementation lands.
type GeminiProvider struct{}

// Name returns "gemini".
func (GeminiProvider) Name() string { return "gemini" }

// ReadInvocations is unimplemented; runHook's panic recover degrades
// the response to "ask" so users see a clear error message.
func (GeminiProvider) ReadInvocations(_ io.Reader) ([]*Invocation, error) {
	panic("provider/gemini: ReadInvocations not implemented")
}

// WriteDecision is unimplemented; see ReadInvocations.
func (GeminiProvider) WriteDecision(_ io.Writer, _ hook.Decision, _ string) error {
	panic("provider/gemini: WriteDecision not implemented")
}
