package llm

import (
	"context"
	"testing"
)

func TestStubProvider_NoResponses(t *testing.T) {
	p := NewStubProvider()
	resp, err := p.Complete(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty response, got %q", resp.Content)
	}
	if p.CallCount() != 1 {
		t.Errorf("expected call count 1, got %d", p.CallCount())
	}
}

func TestStubProvider_CyclesResponses(t *testing.T) {
	p := NewStubProvider("first", "second")

	resp1, err := p.Complete(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1.Content != "first" {
		t.Errorf("expected 'first', got %q", resp1.Content)
	}

	resp2, err := p.Complete(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.Content != "second" {
		t.Errorf("expected 'second', got %q", resp2.Content)
	}

	// Cycles back to first.
	resp3, err := p.Complete(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp3.Content != "first" {
		t.Errorf("expected 'first', got %q", resp3.Content)
	}

	if p.CallCount() != 3 {
		t.Errorf("expected call count 3, got %d", p.CallCount())
	}
}
