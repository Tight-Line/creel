package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider calls the OpenAI chat completions API.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string // overridable for testing; defaults to OpenAI production
}

// NewOpenAIProvider creates an LLM provider that calls OpenAI.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com",
	}
}

// Complete sends a conversation to the OpenAI chat completions API.
func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	apiMessages := make([]map[string]string, len(messages))
	for i, m := range messages {
		apiMessages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	body, err := json.Marshal(map[string]any{
		"model":    p.model,
		"messages": apiMessages,
	})
	if err != nil { // coverage:ignore - json.Marshal of simple map cannot fail
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil { // coverage:ignore - NewRequestWithContext only fails with invalid HTTP method
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI chat completions API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI chat completions API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	return &Response{Content: result.Choices[0].Message.Content}, nil
}
