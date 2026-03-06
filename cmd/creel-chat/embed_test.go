package main

import (
	"testing"
)

func TestNewEmbedder_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	e, err := newEmbedder("openai", "text-embedding-3-small", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Dimension() != 1536 {
		t.Errorf("expected dimension 1536, got %d", e.Dimension())
	}
}

func TestNewEmbedder_OpenAI_MissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := newEmbedder("openai", "text-embedding-3-small", "")
	if err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}

func TestNewEmbedder_Ollama(t *testing.T) {
	e, err := newEmbedder("ollama", "nomic-embed-text", "http://localhost:11434")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Dimension() != 0 {
		t.Errorf("expected dimension 0 for ollama, got %d", e.Dimension())
	}
}

func TestNewEmbedder_Invalid(t *testing.T) {
	_, err := newEmbedder("invalid", "model", "")
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
}
