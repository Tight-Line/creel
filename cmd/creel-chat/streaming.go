package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamToken is a single token from a streaming LLM response.
type StreamToken struct {
	Text string
	Done bool
	Err  error
}

// ParseOpenAIStream reads an OpenAI SSE stream and emits tokens.
// Each SSE line has the format: "data: {json}" where the JSON contains
// choices[0].delta.content. The stream ends with "data: [DONE]".
func ParseOpenAIStream(r io.Reader) <-chan StreamToken {
	ch := make(chan StreamToken, 16)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamToken{Done: true}
				return
			}
			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				ch <- StreamToken{Err: fmt.Errorf("parsing OpenAI SSE event: %w", err)}
				return
			}
			if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
				ch <- StreamToken{Text: event.Choices[0].Delta.Content}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamToken{Err: fmt.Errorf("reading OpenAI SSE stream: %w", err)}
		}
	}()
	return ch
}

// ParseAnthropicStream reads an Anthropic SSE stream and emits tokens.
// Events look like:
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"token"}}
//
// The stream ends with event: message_stop.
func ParseAnthropicStream(r io.Reader) <-chan StreamToken {
	ch := make(chan StreamToken, 16)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		var currentEvent string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if currentEvent == "message_stop" {
					ch <- StreamToken{Done: true}
					return
				}
				if currentEvent == "content_block_delta" {
					var event struct {
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					}
					if err := json.Unmarshal([]byte(data), &event); err != nil {
						ch <- StreamToken{Err: fmt.Errorf("parsing Anthropic SSE event: %w", err)}
						return
					}
					if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
						ch <- StreamToken{Text: event.Delta.Text}
					}
				}
				currentEvent = ""
				continue
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamToken{Err: fmt.Errorf("reading Anthropic SSE stream: %w", err)}
		}
	}()
	return ch
}

// CollectStream reads all tokens from a channel and returns the full text.
// It calls onToken for each token as it arrives (for display).
func CollectStream(tokens <-chan StreamToken, onToken func(string)) (string, error) {
	var sb strings.Builder
	for tok := range tokens {
		if tok.Err != nil {
			return sb.String(), tok.Err
		}
		if tok.Done {
			break
		}
		sb.WriteString(tok.Text)
		if onToken != nil {
			onToken(tok.Text)
		}
	}
	return sb.String(), nil
}
