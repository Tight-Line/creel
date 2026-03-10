package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// mockToolHandler is a ToolHandler with nil clients for testing server protocol.
// We test the MCP protocol layer; tool execution is tested in tools_test.go.

func TestServer_Initialize(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", result["protocolVersion"])
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected serverInfo map")
	}
	if info["name"] != "creel-mcp" {
		t.Errorf("expected server name creel-mcp, got %v", info["name"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	if len(tools) != 10 {
		t.Errorf("expected 10 tools, got %d", len(tools))
	}

	// Verify tool names are present.
	expectedNames := map[string]bool{
		"creel_search":          false,
		"creel_get_context":     false,
		"creel_add_memory":      false,
		"creel_search_memories": false,
		"creel_list_memories":   false,
		"creel_delete_memory":   false,
		"creel_upload_document": false,
		"creel_list_topics":     false,
		"creel_get_document":    false,
		"creel_list_documents":  false,
	}
	for _, tool := range tools {
		tm, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tm["name"].(string)
		expectedNames[name] = true
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestServer_Ping(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":3,"method":"ping"}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":4,"method":"nonexistent"}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestServer_ParseError(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `not valid json` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected code -32700, got %d", resp.Error.Code)
	}
}

func TestServer_ToolsCallUnknownTool(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Errorf("expected 'unknown tool' in error, got %s", resp.Error.Message)
	}
}

func TestServer_ToolsCallInvalidParams(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":"bad"}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

func TestServer_Initialized(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":7,"method":"initialized"}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestServer_MultipleRequests(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	reqs := `{"jsonrpc":"2.0","id":1,"method":"ping"}
{"jsonrpc":"2.0","id":2,"method":"ping"}
`
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(reqs), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}
}

func TestServer_EmptyLines(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	reqs := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(reqs), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d: %v", len(lines), lines)
	}
}

func TestServer_ToolsCallSuccess(t *testing.T) {
	retrieval := &mockRetrievalClient{
		searchResp: &pb.SearchResponse{},
	}
	handler := NewToolHandler("key", &mockTopicClient{listResp: &pb.ListTopicsResponse{}}, &mockDocClient{}, nil, retrieval, &mockMemoryClient{}, &mockJobClient{})
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"creel_search","arguments":{"topic_ids":["t1"],"query_text":"hello"}}}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["isError"] != nil {
		t.Error("expected no isError in success response")
	}
}

func TestServer_ToolsCallError(t *testing.T) {
	retrieval := &mockRetrievalClient{
		searchErr: errors.New("search failed"),
	}
	handler := NewToolHandler("key", &mockTopicClient{}, &mockDocClient{}, nil, retrieval, &mockMemoryClient{}, &mockJobClient{})
	s := NewServer(handler)

	req := `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"creel_search","arguments":{"topic_ids":["t1"],"query_text":"hello"}}}` + "\n"
	var out bytes.Buffer
	err := s.Run(context.Background(), strings.NewReader(req), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	// Tool errors are returned as isError in the result, not as JSON-RPC errors.
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["isError"] != true {
		t.Error("expected isError=true for tool error")
	}
}

func TestServer_ContextCancellation(t *testing.T) {
	handler := &ToolHandler{}
	s := NewServer(handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	// Provide a request that would block if context isn't checked.
	reqs := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	var out bytes.Buffer
	_ = s.Run(ctx, strings.NewReader(reqs), &out)
	// We just verify it doesn't hang.
}
