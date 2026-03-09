package main

import (
	"strings"
	"testing"
)

func TestParseOpenAIStream_Basic(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", text)
	}
}

func TestParseOpenAIStream_EmptyDelta(t *testing.T) {
	// Some events have empty content (e.g., role-only deltas).
	input := `data: {"choices":[{"delta":{"role":"assistant"}}]}
data: {"choices":[{"delta":{"content":"Hi"}}]}
data: [DONE]
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hi" {
		t.Errorf("expected 'Hi', got %q", text)
	}
}

func TestParseOpenAIStream_InvalidJSON(t *testing.T) {
	input := `data: {invalid json}
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	_, err := CollectStream(tokens, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing OpenAI SSE event") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseOpenAIStream_EmptyLines(t *testing.T) {
	// SSE streams typically have blank lines between events.
	input := `data: {"choices":[{"delta":{"content":"A"}}]}

data: {"choices":[{"delta":{"content":"B"}}]}

data: [DONE]
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "AB" {
		t.Errorf("expected 'AB', got %q", text)
	}
}

func TestParseOpenAIStream_NonDataLines(t *testing.T) {
	// Lines that don't start with "data: " should be ignored.
	input := `: comment
id: 123
data: {"choices":[{"delta":{"content":"ok"}}]}
data: [DONE]
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "ok" {
		t.Errorf("expected 'ok', got %q", text)
	}
}

func TestParseOpenAIStream_NoChoices(t *testing.T) {
	input := `data: {"choices":[]}
data: [DONE]
`
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestParseAnthropicStream_Basic(t *testing.T) {
	input := `event: message_start
data: {"type":"message_start"}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" there"}}

event: message_stop
data: {"type":"message_stop"}
`
	tokens := ParseAnthropicStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello there" {
		t.Errorf("expected 'Hello there', got %q", text)
	}
}

func TestParseAnthropicStream_InvalidJSON(t *testing.T) {
	input := `event: content_block_delta
data: {bad json}
`
	tokens := ParseAnthropicStream(strings.NewReader(input))
	_, err := CollectStream(tokens, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing Anthropic SSE event") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAnthropicStream_NonTextDelta(t *testing.T) {
	// Delta types other than text_delta should be ignored.
	input := `event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"foo"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}

event: message_stop
data: {"type":"message_stop"}
`
	tokens := ParseAnthropicStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "ok" {
		t.Errorf("expected 'ok', got %q", text)
	}
}

func TestParseAnthropicStream_OtherEvents(t *testing.T) {
	// Non content_block_delta events with data should be ignored.
	input := `event: content_block_start
data: {"type":"content_block_start","content_block":{"type":"text"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"yes"}}

event: content_block_stop
data: {"type":"content_block_stop"}

event: message_stop
data: {"type":"message_stop"}
`
	tokens := ParseAnthropicStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "yes" {
		t.Errorf("expected 'yes', got %q", text)
	}
}

func TestCollectStream_OnTokenCallback(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"A"}}]}
data: {"choices":[{"delta":{"content":"B"}}]}
data: [DONE]
`
	var collected []string
	tokens := ParseOpenAIStream(strings.NewReader(input))
	text, err := CollectStream(tokens, func(s string) {
		collected = append(collected, s)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "AB" {
		t.Errorf("expected 'AB', got %q", text)
	}
	if len(collected) != 2 || collected[0] != "A" || collected[1] != "B" {
		t.Errorf("expected callback with [A, B], got %v", collected)
	}
}

func TestCollectStream_EmptyStream(t *testing.T) {
	ch := make(chan StreamToken)
	close(ch)
	text, err := CollectStream(ch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestParseAnthropicStream_EmptyTextDelta(t *testing.T) {
	input := `event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}

event: message_stop
data: {"type":"message_stop"}
`
	tokens := ParseAnthropicStream(strings.NewReader(input))
	text, err := CollectStream(tokens, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hi" {
		t.Errorf("expected 'hi', got %q", text)
	}
}
