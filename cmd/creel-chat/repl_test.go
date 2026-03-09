package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

func TestBuildMessages_NoContext(t *testing.T) {
	msgs := buildMessages(nil, nil, "hello", nil)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "hello" {
		t.Errorf("expected user message 'hello', got %s: %s", msgs[1].Role, msgs[1].Content)
	}
}

func TestBuildMessages_WithSessionHistory(t *testing.T) {
	session := []ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "reply"},
	}
	msgs := buildMessages(nil, session, "second", nil)

	// system + 2 session + current user = 4
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[1].Content != "first" {
		t.Errorf("expected session message 'first', got %s", msgs[1].Content)
	}
	if msgs[3].Content != "second" {
		t.Errorf("expected current input 'second', got %s", msgs[3].Content)
	}
}

func TestBuildMessages_WithRetrievedContext(t *testing.T) {
	meta1, _ := structpb.NewStruct(map[string]any{"role": "user", "turn": float64(1)})
	meta2, _ := structpb.NewStruct(map[string]any{"role": "assistant", "turn": float64(1)})

	retrieved := []*pb.SearchResult{
		{
			Chunk: &pb.Chunk{Content: "What is 2+2?", Metadata: meta1},
			Score: 0.9,
		},
		{
			Chunk: &pb.Chunk{Content: "4", Metadata: meta2},
			Score: 0.8,
		},
	}

	msgs := buildMessages(retrieved, nil, "question", nil)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}

	// System prompt should contain the retrieved context.
	sys := msgs[0].Content
	if !contains(sys, "What is 2+2?") {
		t.Error("system prompt should contain retrieved user chunk")
	}
	if !contains(sys, "[assistant]: 4") {
		t.Error("system prompt should contain retrieved assistant chunk")
	}
}

func TestBuildMessages_RetrievedSortedByTurn(t *testing.T) {
	meta1, _ := structpb.NewStruct(map[string]any{"role": "user", "turn": float64(3)})
	meta2, _ := structpb.NewStruct(map[string]any{"role": "assistant", "turn": float64(1)})

	// Provide in reverse order; buildMessages should sort by turn.
	retrieved := []*pb.SearchResult{
		{Chunk: &pb.Chunk{Content: "later", Metadata: meta1}, Score: 0.9},
		{Chunk: &pb.Chunk{Content: "earlier", Metadata: meta2}, Score: 0.8},
	}

	msgs := buildMessages(retrieved, nil, "q", nil)
	sys := msgs[0].Content

	earlierIdx := indexOf(sys, "earlier")
	laterIdx := indexOf(sys, "later")
	if earlierIdx == -1 || laterIdx == -1 {
		t.Fatal("expected both chunks in system prompt")
	}
	if earlierIdx > laterIdx {
		t.Error("expected 'earlier' (turn 1) before 'later' (turn 3) in context")
	}
}

func TestBuildMessages_WithMemories(t *testing.T) {
	memories := []*pb.Memory{
		{Content: "User likes fishing"},
		{Content: "User lives in Maine"},
	}
	msgs := buildMessages(nil, nil, "hello", memories)

	sys := msgs[0].Content
	if !contains(sys, "WHAT I KNOW ABOUT YOU") {
		t.Error("system prompt should contain memory header")
	}
	if !contains(sys, "User likes fishing") {
		t.Error("system prompt should contain first memory")
	}
	if !contains(sys, "User lives in Maine") {
		t.Error("system prompt should contain second memory")
	}
}

func TestBuildMessages_WithCitations(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user", "turn": float64(1)})
	retrieved := []*pb.SearchResult{
		{
			Chunk: &pb.Chunk{Content: "Important fact", Metadata: meta},
			DocumentCitation: &pb.DocumentCitation{
				Name:   "Research Paper",
				Author: "Dr. Smith",
				Url:    "https://example.com/paper",
			},
		},
	}

	msgs := buildMessages(retrieved, nil, "q", nil)
	sys := msgs[0].Content

	if !contains(sys, "[Source:") {
		t.Error("system prompt should contain citation source")
	}
	if !contains(sys, "Research Paper") {
		t.Error("system prompt should contain document name")
	}
	if !contains(sys, "Dr. Smith") {
		t.Error("system prompt should contain author")
	}
}

func TestBuildMessages_WithMemoriesAndContext(t *testing.T) {
	memories := []*pb.Memory{
		{Content: "User is a developer"},
	}
	meta, _ := structpb.NewStruct(map[string]any{"role": "user", "turn": float64(1)})
	retrieved := []*pb.SearchResult{
		{Chunk: &pb.Chunk{Content: "previous convo", Metadata: meta}},
	}

	msgs := buildMessages(retrieved, nil, "q", memories)
	sys := msgs[0].Content

	// Both memory and RAG sections should be present.
	if !contains(sys, "WHAT I KNOW ABOUT YOU") {
		t.Error("system prompt should contain memory section")
	}
	if !contains(sys, "--- RAG CONTEXT") {
		t.Error("system prompt should contain RAG context section")
	}

	// Memory section should come before the RAG context block.
	memIdx := indexOf(sys, "WHAT I KNOW ABOUT YOU")
	ragIdx := indexOf(sys, "--- RAG CONTEXT")
	if memIdx > ragIdx {
		t.Error("memory section should come before RAG context block")
	}
}

func TestExtractTurn_NilMetadata(t *testing.T) {
	r := &pb.SearchResult{Chunk: &pb.Chunk{Content: "test"}}
	if got := extractTurn(r); got != 0 {
		t.Errorf("expected 0 for nil metadata, got %f", got)
	}
}

func TestExtractRole_NilMetadata(t *testing.T) {
	r := &pb.SearchResult{Chunk: &pb.Chunk{Content: "test"}}
	if got := extractRole(r); got != "unknown" {
		t.Errorf("expected 'unknown' for nil metadata, got %s", got)
	}
}

func TestExtractTurn_NilChunk(t *testing.T) {
	r := &pb.SearchResult{}
	if got := extractTurn(r); got != 0 {
		t.Errorf("expected 0 for nil chunk, got %f", got)
	}
}

func TestExtractRole_NilChunk(t *testing.T) {
	r := &pb.SearchResult{}
	if got := extractRole(r); got != "unknown" {
		t.Errorf("expected 'unknown' for nil chunk, got %s", got)
	}
}

func TestExtractTurn_NoTurnField(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user"})
	r := &pb.SearchResult{Chunk: &pb.Chunk{Metadata: meta}}
	if got := extractTurn(r); got != 0 {
		t.Errorf("expected 0 for missing turn field, got %f", got)
	}
}

func TestExtractRole_NoRoleField(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"turn": float64(1)})
	r := &pb.SearchResult{Chunk: &pb.Chunk{Metadata: meta}}
	if got := extractRole(r); got != "unknown" {
		t.Errorf("expected 'unknown' for missing role field, got %s", got)
	}
}

func TestBuildMessages_NilChunkInRetrieved(t *testing.T) {
	retrieved := []*pb.SearchResult{
		{Chunk: nil},
	}
	msgs := buildMessages(retrieved, nil, "q", nil)
	sys := msgs[0].Content
	// Should not contain the RAG CONTEXT block delimiter since the only chunk was nil.
	if strings.Contains(sys, "--- RAG CONTEXT") {
		t.Error("should not include RAG CONTEXT block when all chunks are nil")
	}
}

func contains(s, substr string) bool {
	return indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
