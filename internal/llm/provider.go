// Package llm provides interfaces and implementations for LLM interactions.
package llm

import "context"

// Message represents a single message in a conversation with an LLM.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// Response represents the LLM's response.
type Response struct {
	Content string
}

// Provider is the interface for completing LLM conversations.
type Provider interface {
	// Complete sends a conversation to the LLM and returns its response.
	Complete(ctx context.Context, messages []Message) (*Response, error)
}

// StubProvider returns configurable responses for testing.
type StubProvider struct {
	responses []string
	callCount int
}

// NewStubProvider creates a stub provider that cycles through the given responses.
// If no responses are provided, it returns an empty string.
func NewStubProvider(responses ...string) *StubProvider {
	return &StubProvider{responses: responses}
}

// Complete returns the next configured response.
func (s *StubProvider) Complete(_ context.Context, _ []Message) (*Response, error) {
	s.callCount++
	if len(s.responses) == 0 {
		return &Response{Content: ""}, nil
	}
	resp := s.responses[(s.callCount-1)%len(s.responses)]
	return &Response{Content: resp}, nil
}

// CallCount returns the number of times Complete was called.
func (s *StubProvider) CallCount() int {
	return s.callCount
}
