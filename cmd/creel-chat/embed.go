package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Embedder computes vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	Dimension() int
}

func newEmbedder(provider, model, ollamaURL string) (Embedder, error) {
	switch provider {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required for openai embeddings")
		}
		return &openaiEmbedder{apiKey: key, model: model}, nil
	case "ollama":
		return &ollamaEmbedder{baseURL: ollamaURL, model: model}, nil
	default:
		return nil, fmt.Errorf("unsupported embed provider: %s (use openai or ollama)", provider)
	}
}

// openaiEmbedder calls the OpenAI embeddings API.
type openaiEmbedder struct {
	apiKey string
	model  string
}

func (e *openaiEmbedder) Dimension() int { return 1536 }

func (e *openaiEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI embeddings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI embeddings API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("OpenAI returned no embeddings")
	}
	return result.Data[0].Embedding, nil
}

// ollamaEmbedder calls a local Ollama instance.
type ollamaEmbedder struct {
	baseURL string
	model   string
}

func (e *ollamaEmbedder) Dimension() int { return 0 } // model-dependent; user must match

func (e *ollamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Ollama embed API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama embed API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("Ollama returned no embeddings")
	}
	return result.Embeddings[0], nil
}
