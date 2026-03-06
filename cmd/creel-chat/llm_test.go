package main

import (
	"testing"
)

func TestNewLLM_Anthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	llm, err := newLLM("anthropic", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm == nil {
		t.Fatal("expected non-nil LLM")
	}
}

func TestNewLLM_Anthropic_MissingKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := newLLM("anthropic", "")
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}

func TestNewLLM_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	llm, err := newLLM("openai", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm == nil {
		t.Fatal("expected non-nil LLM")
	}
}

func TestNewLLM_OpenAI_MissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := newLLM("openai", "")
	if err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}

func TestNewLLM_Invalid(t *testing.T) {
	_, err := newLLM("invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
}

func TestNewLLM_CustomModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	llm, err := newLLM("anthropic", "claude-opus-4-20250514")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := llm.(*anthropicLLM)
	if !ok {
		t.Fatal("expected *anthropicLLM")
	}
	if a.model != "claude-opus-4-20250514" {
		t.Errorf("expected custom model, got %s", a.model)
	}
}
