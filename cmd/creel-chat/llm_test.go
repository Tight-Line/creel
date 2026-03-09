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

func TestBuildOpenAIMessages(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
	}
	api := buildOpenAIMessages(msgs)
	if len(api) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(api))
	}
	if api[0]["role"] != "system" || api[0]["content"] != "sys" {
		t.Errorf("first message mismatch: %v", api[0])
	}
	if api[1]["role"] != "user" || api[1]["content"] != "hi" {
		t.Errorf("second message mismatch: %v", api[1])
	}
}

func TestBuildAnthropicRequest_NoStream(t *testing.T) {
	l := &anthropicLLM{apiKey: "key", model: "model"}
	msgs := []ChatMessage{
		{Role: "system", Content: "sys prompt"},
		{Role: "user", Content: "hello"},
	}
	reqBody, _ := l.buildAnthropicRequest(msgs, false)

	if reqBody["model"] != "model" {
		t.Errorf("expected model 'model', got %v", reqBody["model"])
	}
	if reqBody["system"] != "sys prompt" {
		t.Errorf("expected system prompt, got %v", reqBody["system"])
	}
	if _, ok := reqBody["stream"]; ok {
		t.Error("should not have stream field when not streaming")
	}
}

func TestBuildAnthropicRequest_WithStream(t *testing.T) {
	l := &anthropicLLM{apiKey: "key", model: "model"}
	msgs := []ChatMessage{
		{Role: "user", Content: "hello"},
	}
	reqBody, _ := l.buildAnthropicRequest(msgs, true)

	if reqBody["stream"] != true {
		t.Error("expected stream=true")
	}
	// No system message, so system key should be absent.
	if _, ok := reqBody["system"]; ok {
		t.Error("should not have system field when no system message")
	}
}

func TestBuildAnthropicRequest_FiltersSystem(t *testing.T) {
	l := &anthropicLLM{apiKey: "key", model: "model"}
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	reqBody, apiMsgs := l.buildAnthropicRequest(msgs, false)

	if reqBody["system"] != "sys" {
		t.Errorf("expected system in reqBody, got %v", reqBody["system"])
	}
	// apiMessages should not include the system message.
	if len(apiMsgs) != 2 {
		t.Errorf("expected 2 api messages (no system), got %d", len(apiMsgs))
	}
}

func TestLLMInterface_ImplementedByBoth(t *testing.T) {
	// Compile-time check that both types implement the LLM interface.
	var _ LLM = (*anthropicLLM)(nil)
	var _ LLM = (*openaiLLM)(nil)
}
