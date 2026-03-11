package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestProvider(t *testing.T, handler http.HandlerFunc) *OpenAIEmbeddingProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := NewOpenAIEmbeddingProvider("test-key", "text-embedding-3-small", 3)
	p.baseURL = srv.URL
	return p
}

func TestOpenAIEmbeddingProvider_Embed(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("unexpected model: %s", req.Model)
		}
		if len(req.Input) != 2 {
			t.Fatalf("expected 2 inputs, got %d", len(req.Input))
		}

		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
				{"index": 1, "embedding": []float64{0.4, 0.5, 0.6}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	embeddings, err := p.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if embeddings[0][0] != 0.1 {
		t.Errorf("embeddings[0][0] = %f, want 0.1", embeddings[0][0])
	}
	if embeddings[1][0] != 0.4 {
		t.Errorf("embeddings[1][0] = %f, want 0.4", embeddings[1][0])
	}
}

func TestOpenAIEmbeddingProvider_Dimensions(t *testing.T) {
	p := NewOpenAIEmbeddingProvider("key", "model", 1536)
	if p.Dimensions() != 1536 {
		t.Errorf("Dimensions() = %d, want 1536", p.Dimensions())
	}
}

func TestOpenAIEmbeddingProvider_HTTPError(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid key"}}`))
	})

	_, err := p.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestOpenAIEmbeddingProvider_CountMismatch(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	_, err := p.Embed(context.Background(), []string{"hello", "world"})
	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestOpenAIEmbeddingProvider_OutOfRangeIndex(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 99, "embedding": []float64{0.1, 0.2, 0.3}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	_, err := p.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestOpenAIEmbeddingProvider_ConnectionError(t *testing.T) {
	p := NewOpenAIEmbeddingProvider("key", "model", 3)
	p.baseURL = "http://127.0.0.1:0" // nothing listening
	_, err := p.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestOpenAIEmbeddingProvider_InvalidJSON(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json`))
	})

	_, err := p.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOpenAIEmbeddingProvider_Model(t *testing.T) {
	p := NewOpenAIEmbeddingProvider("key", "text-embedding-3-small", 1536)
	if got := p.Model(); got != "text-embedding-3-small" {
		t.Errorf("Model() = %q, want %q", got, "text-embedding-3-small")
	}
}

func TestOpenAIEmbeddingProvider_Interface(t *testing.T) {
	var _ EmbeddingProvider = &OpenAIEmbeddingProvider{}
}
