package worker

import (
	"context"
	"errors"
	"testing"
)

func TestStubEmbeddingProvider_Embed(t *testing.T) {
	p := NewStubEmbeddingProvider(4)
	embs, err := p.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embs) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embs))
	}
	for i, emb := range embs {
		if len(emb) != 4 {
			t.Errorf("embedding[%d] dim = %d, want 4", i, len(emb))
		}
	}
}

func TestStubEmbeddingProvider_EmptyText(t *testing.T) {
	p := NewStubEmbeddingProvider(3)
	embs, err := p.Embed(context.Background(), []string{""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embs) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(embs))
	}
	if len(embs[0]) != 3 {
		t.Errorf("dimension = %d, want 3", len(embs[0]))
	}
	// All elements should be 0.01 for empty text.
	for _, v := range embs[0] {
		if v != 0.01 {
			t.Errorf("expected 0.01 for empty text, got %f", v)
			break
		}
	}
}

func TestStubEmbeddingProvider_Dimensions(t *testing.T) {
	p := NewStubEmbeddingProvider(1536)
	if p.Dimensions() != 1536 {
		t.Errorf("Dimensions() = %d, want 1536", p.Dimensions())
	}
}

func TestStubEmbeddingProvider_Deterministic(t *testing.T) {
	p := NewStubEmbeddingProvider(4)
	embs1, _ := p.Embed(context.Background(), []string{"test"})
	embs2, _ := p.Embed(context.Background(), []string{"test"})
	for i := range embs1[0] {
		if embs1[0][i] != embs2[0][i] {
			t.Errorf("embeddings are not deterministic at index %d", i)
		}
	}
}

func TestEmbeddingWorker_Type(t *testing.T) {
	w := NewEmbeddingWorker(nil, nil, nil, nil, nil, nil)
	if w.Type() != "embedding" {
		t.Errorf("Type() = %q, want embedding", w.Type())
	}
}

// failingEmbeddingProvider always returns an error.
type failingEmbeddingProvider struct {
	dim int
}

func (p *failingEmbeddingProvider) Embed(_ context.Context, _ []string) ([][]float64, error) {
	return nil, errors.New("embedding service unavailable")
}

func (p *failingEmbeddingProvider) Dimensions() int {
	return p.dim
}

// mismatchEmbeddingProvider returns wrong number of embeddings.
type mismatchEmbeddingProvider struct {
	dim int
}

func (p *mismatchEmbeddingProvider) Embed(_ context.Context, _ []string) ([][]float64, error) {
	// Return only one embedding regardless of input count.
	return [][]float64{make([]float64, p.dim)}, nil
}

func (p *mismatchEmbeddingProvider) Dimensions() int {
	return p.dim
}
