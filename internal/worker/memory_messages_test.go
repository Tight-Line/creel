package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
)

func TestMemoryMessagesWorker_Type(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, nil)
	if w.Type() != "memory_messages" {
		t.Errorf("expected 'memory_messages', got %q", w.Type())
	}
}

func TestMemoryMessagesWorker_MissingPrincipal(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, nil)
	job := &store.ProcessingJob{Progress: map[string]any{
		"messages": []any{},
	}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "missing principal") {
		t.Fatalf("expected missing principal error, got: %v", err)
	}
}

func TestMemoryMessagesWorker_MissingMessages(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, nil)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
	}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "missing messages") {
		t.Fatalf("expected missing messages error, got: %v", err)
	}
}

func TestMemoryMessagesWorker_EmptyMessages(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, nil)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages":  []any{},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error for empty messages: %v", err)
	}
}

func TestMemoryMessagesWorker_DefaultScope(t *testing.T) {
	jobDB := &mockJobWithProgressDBTX{}
	w := NewMemoryMessagesWorker(
		store.NewJobStore(jobDB),
		llm.NewStubProvider(`{"facts": ["user likes cats"]}`),
	)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		// no scope; should default to "default"
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMessagesWorker_LLMError(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, &failingLLMProvider{})
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "calling LLM for fact extraction") {
		t.Fatalf("expected LLM error, got: %v", err)
	}
}

func TestMemoryMessagesWorker_BadJSON(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, llm.NewStubProvider("not json"))
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "parsing extraction response") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestMemoryMessagesWorker_NoFacts(t *testing.T) {
	w := NewMemoryMessagesWorker(nil, llm.NewStubProvider(`{"facts": []}`))
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "Hello"},
			map[string]any{"role": "assistant", "content": "Hi there!"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMessagesWorker_MultipleFacts(t *testing.T) {
	jobDB := &mockJobWithProgressDBTX{}
	w := NewMemoryMessagesWorker(
		store.NewJobStore(jobDB),
		llm.NewStubProvider(`{"facts": ["user likes cats", "user lives in Maine"]}`),
	)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats and I live in Maine"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMessagesWorker_SkipsEmptyFacts(t *testing.T) {
	jobDB := &mockJobWithProgressDBTX{}
	w := NewMemoryMessagesWorker(
		store.NewJobStore(jobDB),
		llm.NewStubProvider(`{"facts": ["user likes cats", "", "user lives in Maine"]}`),
	)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats and I live in Maine"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMessagesWorker_CreateJobError(t *testing.T) {
	// Use a DBTX that fails on QueryRow (which CreateDocless uses).
	failDB := &mockMemoryDBTXWithFailCreate{}
	w := NewMemoryMessagesWorker(
		store.NewJobStore(failDB),
		llm.NewStubProvider(`{"facts": ["user likes cats"]}`),
	)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I like cats"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "creating memory_maintenance job") {
		t.Fatalf("expected create job error, got: %v", err)
	}
}

func TestMemoryMessagesWorker_CodeFenceJSON(t *testing.T) {
	// LLM wraps JSON in markdown code fences; extractJSON should handle it.
	jobDB := &mockJobWithProgressDBTX{}
	w := NewMemoryMessagesWorker(
		store.NewJobStore(jobDB),
		llm.NewStubProvider("```json\n{\"facts\": [\"user likes fishing\"]}\n```"),
	)
	job := &store.ProcessingJob{Progress: map[string]any{
		"principal": "user1",
		"scope":     "test",
		"messages": []any{
			map[string]any{"role": "user", "content": "I love fly fishing"},
		},
	}}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
