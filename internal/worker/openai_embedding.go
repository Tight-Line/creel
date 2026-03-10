package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIEmbeddingProvider computes embeddings via the OpenAI API.
type OpenAIEmbeddingProvider struct {
	apiKey  string
	model   string
	dim     int
	baseURL string // overridable for testing; defaults to OpenAI production
}

// NewOpenAIEmbeddingProvider creates an embedding provider that calls OpenAI.
func NewOpenAIEmbeddingProvider(apiKey, model string, dimensions int) *OpenAIEmbeddingProvider {
	return &OpenAIEmbeddingProvider{
		apiKey:  apiKey,
		model:   model,
		dim:     dimensions,
		baseURL: "https://api.openai.com",
	}
}

// Embed computes embeddings for the given texts via the OpenAI API.
func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	body, err := json.Marshal(map[string]any{
		"model": p.model,
		"input": texts,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI embeddings API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI embeddings API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("OpenAI returned %d embeddings, expected %d", len(result.Data), len(texts))
	}

	// OpenAI returns embeddings in the same order as input, but sort by index to be safe.
	embeddings := make([][]float64, len(texts))
	for _, d := range result.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("OpenAI returned out-of-range index %d", d.Index)
		}
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// Dimensions returns the embedding dimension.
func (p *OpenAIEmbeddingProvider) Dimensions() int {
	return p.dim
}
