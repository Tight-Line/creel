package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// Mock gRPC clients for testing tool execution.

type mockTopicClient struct {
	pb.TopicServiceClient
	listResp *pb.ListTopicsResponse
	listErr  error
}

func (m *mockTopicClient) ListTopics(_ context.Context, _ *pb.ListTopicsRequest, _ ...grpc.CallOption) (*pb.ListTopicsResponse, error) {
	return m.listResp, m.listErr
}

type mockDocClient struct {
	pb.DocumentServiceClient
	getResp    *pb.Document
	getErr     error
	listResp   *pb.ListDocumentsResponse
	listErr    error
	uploadResp *pb.UploadDocumentResponse
	uploadErr  error
}

func (m *mockDocClient) GetDocument(_ context.Context, _ *pb.GetDocumentRequest, _ ...grpc.CallOption) (*pb.Document, error) {
	return m.getResp, m.getErr
}

func (m *mockDocClient) ListDocuments(_ context.Context, _ *pb.ListDocumentsRequest, _ ...grpc.CallOption) (*pb.ListDocumentsResponse, error) {
	return m.listResp, m.listErr
}

func (m *mockDocClient) UploadDocument(_ context.Context, _ *pb.UploadDocumentRequest, _ ...grpc.CallOption) (*pb.UploadDocumentResponse, error) {
	return m.uploadResp, m.uploadErr
}

type mockRetrievalClient struct {
	pb.RetrievalServiceClient
	searchResp  *pb.SearchResponse
	searchErr   error
	contextResp *pb.GetContextResponse
	contextErr  error
}

func (m *mockRetrievalClient) Search(_ context.Context, _ *pb.SearchRequest, _ ...grpc.CallOption) (*pb.SearchResponse, error) {
	return m.searchResp, m.searchErr
}

func (m *mockRetrievalClient) GetContext(_ context.Context, _ *pb.GetContextRequest, _ ...grpc.CallOption) (*pb.GetContextResponse, error) {
	return m.contextResp, m.contextErr
}

type mockMemoryClient struct {
	pb.MemoryServiceClient
	addResp    *pb.Memory
	addErr     error
	searchResp *pb.SearchMemoriesResponse
	searchErr  error
	listResp   *pb.ListMemoriesResponse
	listErr    error
	deleteErr  error
}

func (m *mockMemoryClient) AddMemory(_ context.Context, _ *pb.AddMemoryRequest, _ ...grpc.CallOption) (*pb.Memory, error) {
	return m.addResp, m.addErr
}

func (m *mockMemoryClient) SearchMemories(_ context.Context, _ *pb.SearchMemoriesRequest, _ ...grpc.CallOption) (*pb.SearchMemoriesResponse, error) {
	return m.searchResp, m.searchErr
}

func (m *mockMemoryClient) ListMemories(_ context.Context, _ *pb.ListMemoriesRequest, _ ...grpc.CallOption) (*pb.ListMemoriesResponse, error) {
	return m.listResp, m.listErr
}

func (m *mockMemoryClient) DeleteMemory(_ context.Context, _ *pb.DeleteMemoryRequest, _ ...grpc.CallOption) (*pb.DeleteMemoryResponse, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	return &pb.DeleteMemoryResponse{}, nil
}

type mockJobClient struct {
	pb.JobServiceClient
}

func newTestHandler() (*ToolHandler, *mockRetrievalClient, *mockMemoryClient, *mockDocClient, *mockTopicClient) {
	retrieval := &mockRetrievalClient{
		searchResp:  &pb.SearchResponse{},
		contextResp: &pb.GetContextResponse{},
	}
	memory := &mockMemoryClient{
		addResp:    &pb.Memory{Id: "m1", Scope: "test", Content: "hello"},
		searchResp: &pb.SearchMemoriesResponse{},
		listResp:   &pb.ListMemoriesResponse{},
	}
	docs := &mockDocClient{
		getResp:    &pb.Document{Id: "d1", Name: "test doc"},
		listResp:   &pb.ListDocumentsResponse{},
		uploadResp: &pb.UploadDocumentResponse{Document: &pb.Document{Id: "d2"}, JobId: "j1"},
	}
	topics := &mockTopicClient{
		listResp: &pb.ListTopicsResponse{},
	}
	h := NewToolHandler("test-key", topics, docs, nil, retrieval, memory, &mockJobClient{})
	return h, retrieval, memory, docs, topics
}

func TestToolHandler_Search(t *testing.T) {
	h, retrieval, _, _, _ := newTestHandler()
	meta, _ := structpb.NewStruct(nil)
	retrieval.searchResp = &pb.SearchResponse{
		Results: []*pb.SearchResult{
			{Chunk: &pb.Chunk{Id: "c1", Content: "found it"}, Score: 0.95, DocumentId: "d1"},
		},
	}
	_ = meta

	result, err := h.Execute(context.Background(), "creel_search", json.RawMessage(`{"topic_ids":["t1"],"query_text":"test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "found it") {
		t.Errorf("expected result to contain chunk content, got: %s", result)
	}
}

func TestToolHandler_SearchError(t *testing.T) {
	h, retrieval, _, _, _ := newTestHandler()
	retrieval.searchErr = errors.New("search failed")

	_, err := h.Execute(context.Background(), "creel_search", json.RawMessage(`{"topic_ids":["t1"],"query_text":"test"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_SearchInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_search", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error for invalid args")
	}
}

func TestToolHandler_GetContext(t *testing.T) {
	h, retrieval, _, _, _ := newTestHandler()
	retrieval.contextResp = &pb.GetContextResponse{
		Chunks: []*pb.Chunk{{Id: "c1", Content: "context chunk"}},
	}

	result, err := h.Execute(context.Background(), "creel_get_context", json.RawMessage(`{"document_id":"d1","last_n":5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "context chunk") {
		t.Errorf("expected context chunk in result, got: %s", result)
	}
}

func TestToolHandler_GetContextWithSince(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	result, err := h.Execute(context.Background(), "creel_get_context", json.RawMessage(`{"document_id":"d1","since":"2024-01-01T00:00:00Z"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestToolHandler_GetContextBadSince(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_get_context", json.RawMessage(`{"document_id":"d1","since":"not-a-date"}`))
	if err == nil {
		t.Fatal("expected error for bad timestamp")
	}
}

func TestToolHandler_AddMemory(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.addResp = &pb.Memory{Id: "m1", Scope: "prefs", Content: "likes fishing"}

	result, err := h.Execute(context.Background(), "creel_add_memory", json.RawMessage(`{"scope":"prefs","content":"likes fishing"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "likes fishing") {
		t.Errorf("expected memory content in result, got: %s", result)
	}
}

func TestToolHandler_SearchMemories(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.searchResp = &pb.SearchMemoriesResponse{
		Results: []*pb.MemorySearchResult{
			{Memory: &pb.Memory{Id: "m1", Content: "found memory"}, Score: 0.8},
		},
	}

	result, err := h.Execute(context.Background(), "creel_search_memories", json.RawMessage(`{"query_text":"fish"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "found memory") {
		t.Errorf("expected memory in result, got: %s", result)
	}
}

func TestToolHandler_ListMemories(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.listResp = &pb.ListMemoriesResponse{
		Memories: []*pb.Memory{{Id: "m1", Scope: "test", Content: "item"}},
	}

	result, err := h.Execute(context.Background(), "creel_list_memories", json.RawMessage(`{"scope":"test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "item") {
		t.Errorf("expected memory item in result, got: %s", result)
	}
}

func TestToolHandler_DeleteMemory(t *testing.T) {
	h, _, _, _, _ := newTestHandler()

	result, err := h.Execute(context.Background(), "creel_delete_memory", json.RawMessage(`{"id":"m1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "deleted") {
		t.Errorf("expected deleted status, got: %s", result)
	}
}

func TestToolHandler_UploadDocument(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.uploadResp = &pb.UploadDocumentResponse{
		Document: &pb.Document{Id: "d2", Name: "uploaded"},
		JobId:    "j1",
	}

	result, err := h.Execute(context.Background(), "creel_upload_document", json.RawMessage(`{"topic_id":"t1","name":"test","content":"hello world"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "uploaded") {
		t.Errorf("expected document name in result, got: %s", result)
	}
}

func TestToolHandler_ListTopics(t *testing.T) {
	h, _, _, _, topics := newTestHandler()
	topics.listResp = &pb.ListTopicsResponse{
		Topics: []*pb.Topic{{Id: "t1", Slug: "my-topic", Name: "My Topic"}},
	}

	result, err := h.Execute(context.Background(), "creel_list_topics", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "my-topic") {
		t.Errorf("expected topic slug in result, got: %s", result)
	}
}

func TestToolHandler_GetDocument(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.getResp = &pb.Document{Id: "d1", Name: "fetched doc"}

	result, err := h.Execute(context.Background(), "creel_get_document", json.RawMessage(`{"id":"d1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "fetched doc") {
		t.Errorf("expected doc name in result, got: %s", result)
	}
}

func TestToolHandler_ListDocuments(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.listResp = &pb.ListDocumentsResponse{
		Documents: []*pb.Document{{Id: "d1", Name: "doc1"}},
	}

	result, err := h.Execute(context.Background(), "creel_list_documents", json.RawMessage(`{"topic_id":"t1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "doc1") {
		t.Errorf("expected document name in result, got: %s", result)
	}
}

func TestToolHandler_GetContextError(t *testing.T) {
	h, retrieval, _, _, _ := newTestHandler()
	retrieval.contextErr = errors.New("context failed")
	_, err := h.Execute(context.Background(), "creel_get_context", json.RawMessage(`{"document_id":"d1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_GetContextInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_get_context", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_AddMemoryError(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.addErr = errors.New("add failed")
	_, err := h.Execute(context.Background(), "creel_add_memory", json.RawMessage(`{"scope":"test","content":"hi"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_AddMemoryInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_add_memory", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_SearchMemoriesError(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.searchErr = errors.New("search failed")
	_, err := h.Execute(context.Background(), "creel_search_memories", json.RawMessage(`{"query_text":"test"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_SearchMemoriesInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_search_memories", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_ListMemoriesError(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.listErr = errors.New("list failed")
	_, err := h.Execute(context.Background(), "creel_list_memories", json.RawMessage(`{"scope":"test"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_ListMemoriesInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_list_memories", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_DeleteMemoryError(t *testing.T) {
	h, _, memory, _, _ := newTestHandler()
	memory.deleteErr = errors.New("delete failed")
	_, err := h.Execute(context.Background(), "creel_delete_memory", json.RawMessage(`{"id":"m1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_DeleteMemoryInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_delete_memory", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_UploadDocumentError(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.uploadErr = errors.New("upload failed")
	_, err := h.Execute(context.Background(), "creel_upload_document", json.RawMessage(`{"topic_id":"t1","name":"test","content":"hi"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_UploadDocumentInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_upload_document", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_ListTopicsError(t *testing.T) {
	h, _, _, _, topics := newTestHandler()
	topics.listErr = errors.New("list failed")
	_, err := h.Execute(context.Background(), "creel_list_topics", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_GetDocumentError(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.getErr = errors.New("get failed")
	_, err := h.Execute(context.Background(), "creel_get_document", json.RawMessage(`{"id":"d1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_GetDocumentInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_get_document", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_ListDocumentsError(t *testing.T) {
	h, _, _, docs, _ := newTestHandler()
	docs.listErr = errors.New("list failed")
	_, err := h.Execute(context.Background(), "creel_list_documents", json.RawMessage(`{"topic_id":"t1"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_ListDocumentsInvalidArgs(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "creel_list_documents", json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolHandler_UnknownTool(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	_, err := h.Execute(context.Background(), "nonexistent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestToolHandler_AuthContext(t *testing.T) {
	h := &ToolHandler{apiKey: "test-key"}
	ctx := h.authCtx(context.Background())
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

// Verify unused imports are used.
var _ = timestamppb.Now
var _ = structpb.NewStruct
