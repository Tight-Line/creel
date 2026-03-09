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

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// LLM generates chat completions.
type LLM interface {
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
	// ChatStream sends a streaming request and returns a channel of tokens.
	// The caller must consume the channel to completion.
	ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamToken, error)
}

func newLLM(provider, model string) (LLM, error) {
	switch provider {
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for anthropic provider")
		}
		m := model
		if m == "" {
			m = "claude-sonnet-4-6"
		}
		return &anthropicLLM{apiKey: key, model: m}, nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
		}
		m := model
		if m == "" {
			m = "gpt-5.4"
		}
		return &openaiLLM{apiKey: key, model: m}, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (use anthropic or openai)", provider)
	}
}

// anthropicLLM calls the Anthropic Messages API.
type anthropicLLM struct {
	apiKey string
	model  string
}

// buildAnthropicRequest builds the HTTP request body and headers for an Anthropic API call.
func (l *anthropicLLM) buildAnthropicRequest(messages []ChatMessage, stream bool) (map[string]any, []map[string]string) {
	var system string
	var apiMessages []map[string]string
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		apiMessages = append(apiMessages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	reqBody := map[string]any{
		"model":      l.model,
		"max_tokens": 4096,
		"messages":   apiMessages,
	}
	if system != "" {
		reqBody["system"] = system
	}
	if stream {
		reqBody["stream"] = true
	}

	return reqBody, apiMessages
}

func (l *anthropicLLM) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	reqBody, _ := l.buildAnthropicRequest(messages, false)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", l.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic returned no text content")
}

func (l *anthropicLLM) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamToken, error) {
	reqBody, _ := l.buildAnthropicRequest(messages, true)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", l.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Anthropic API: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, b)
	}

	// The body is kept open; ParseAnthropicStream reads it in a goroutine.
	// The goroutine closes implicitly when the body is drained.
	return ParseAnthropicStream(resp.Body), nil
}

// openaiLLM calls the OpenAI Chat Completions API.
type openaiLLM struct {
	apiKey string
	model  string
}

// buildOpenAIMessages converts ChatMessages to the OpenAI API format.
func buildOpenAIMessages(messages []ChatMessage) []map[string]string {
	var apiMessages []map[string]string
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	return apiMessages
}

func (l *openaiLLM) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	apiMessages := buildOpenAIMessages(messages)

	body, err := json.Marshal(map[string]any{
		"model":    l.model,
		"messages": apiMessages,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling OpenAI API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

func (l *openaiLLM) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamToken, error) {
	apiMessages := buildOpenAIMessages(messages)

	body, err := json.Marshal(map[string]any{
		"model":    l.model,
		"messages": apiMessages,
		"stream":   true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI API: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("OpenAI API returned %d: %s", resp.StatusCode, b)
	}

	return ParseOpenAIStream(resp.Body), nil
}
