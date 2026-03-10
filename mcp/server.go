// Package mcp implements a Model Context Protocol (MCP) server for Creel.
//
// The server exposes Creel operations as MCP tools, allowing AI agents
// to interact with the Creel memory platform via the MCP stdio protocol.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Server is an MCP server that exposes Creel tools.
type Server struct {
	tools   map[string]Tool
	handler *ToolHandler
	mu      sync.Mutex
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// jsonrpcRequest is a JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is a JSON-RPC 2.0 response.
type jsonrpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonrpcError `json:"error,omitempty"`
}

// jsonrpcError is a JSON-RPC 2.0 error.
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates a new MCP server backed by the given tool handler.
func NewServer(handler *ToolHandler) *Server {
	s := &Server{
		tools:   make(map[string]Tool),
		handler: handler,
	}
	s.registerTools()
	return s
}

// Run reads JSON-RPC requests from r and writes responses to w.
// It blocks until r is closed or ctx is cancelled.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(w, jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := s.handleRequest(ctx, &req)
		s.writeResponse(w, resp)
	}
	return scanner.Err()
}

// RunStdio is a convenience method that runs the server on stdin/stdout.
// coverage:ignore - wraps Run with os.Stdin/Stdout; tested via Run directly
func (s *Server) RunStdio(ctx context.Context) error {
	return s.Run(ctx, os.Stdin, os.Stdout)
}

func (s *Server) handleRequest(ctx context.Context, req *jsonrpcRequest) jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification; no response needed, but return empty for safety.
		return jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req *jsonrpcRequest) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "creel-mcp",
				"version": "0.5.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req *jsonrpcRequest) jsonrpcResponse {
	tools := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req *jsonrpcRequest) jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "invalid params"},
		}
	}

	if _, ok := s.tools[params.Name]; !ok {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}

	result, err := s.handler.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			},
		}
	}

	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": result},
			},
		},
	}
}

func (s *Server) writeResponse(w io.Writer, resp jsonrpcResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", data)
}

func (s *Server) registerTools() {
	defs := []struct {
		name, desc string
		schema     string
	}{
		{
			"creel_search",
			"Search for relevant content chunks across one or more topics. Returns matching chunks with relevance scores and optional citation metadata.",
			`{"type":"object","properties":{"topic_ids":{"type":"array","items":{"type":"string"},"description":"Topic IDs or slugs to search"},"query_text":{"type":"string","description":"Natural language search query"},"top_k":{"type":"integer","description":"Maximum results to return (default 10)"},"follow_links":{"type":"boolean","description":"Follow chunk links to find related content"},"link_depth":{"type":"integer","description":"Maximum link traversal depth"}},"required":["topic_ids","query_text"]}`,
		},
		{
			"creel_get_context",
			"Get temporal context from a document. Returns recent chunks in chronological order, useful for understanding conversation history or document timeline.",
			`{"type":"object","properties":{"document_id":{"type":"string","description":"Document ID"},"last_n":{"type":"integer","description":"Number of recent chunks to return"},"since":{"type":"string","description":"ISO 8601 timestamp; return chunks created after this time"}},"required":["document_id"]}`,
		},
		{
			"creel_add_memory",
			"Store a memory observation. Memories persist across sessions and are scoped to the calling principal and a named scope.",
			`{"type":"object","properties":{"scope":{"type":"string","description":"Memory scope name (e.g. 'preferences', 'context')"},"content":{"type":"string","description":"The memory content to store"},"subject":{"type":"string","description":"Subject of the observation (optional SPO triple)"},"predicate":{"type":"string","description":"Predicate of the observation"},"object":{"type":"string","description":"Object of the observation"}},"required":["scope","content"]}`,
		},
		{
			"creel_search_memories",
			"Search stored memories by semantic similarity. Returns memories ranked by relevance to the query.",
			`{"type":"object","properties":{"query_text":{"type":"string","description":"Natural language search query"},"scope":{"type":"string","description":"Limit search to a specific scope"},"top_k":{"type":"integer","description":"Maximum results to return (default 10)"}},"required":["query_text"]}`,
		},
		{
			"creel_list_memories",
			"List all memories in a given scope.",
			`{"type":"object","properties":{"scope":{"type":"string","description":"Memory scope name"},"include_invalidated":{"type":"boolean","description":"Include soft-deleted memories"}},"required":["scope"]}`,
		},
		{
			"creel_delete_memory",
			"Delete a memory by ID. This soft-deletes the memory.",
			`{"type":"object","properties":{"id":{"type":"string","description":"Memory ID to delete"}},"required":["id"]}`,
		},
		{
			"creel_upload_document",
			"Upload text content as a new document for processing. Creel will extract, chunk, and embed it automatically.",
			`{"type":"object","properties":{"topic_id":{"type":"string","description":"Topic ID or slug"},"name":{"type":"string","description":"Document name"},"content":{"type":"string","description":"Text content to upload"},"slug":{"type":"string","description":"Optional URL-friendly slug"},"content_type":{"type":"string","description":"Content type (default text/plain)"}},"required":["topic_id","name","content"]}`,
		},
		{
			"creel_list_topics",
			"List available topics that the current principal has access to.",
			`{"type":"object","properties":{}}`,
		},
		{
			"creel_get_document",
			"Get metadata for a specific document.",
			`{"type":"object","properties":{"id":{"type":"string","description":"Document ID"}},"required":["id"]}`,
		},
		{
			"creel_list_documents",
			"List documents in a topic.",
			`{"type":"object","properties":{"topic_id":{"type":"string","description":"Topic ID or slug"}},"required":["topic_id"]}`,
		},
	}

	for _, d := range defs {
		s.tools[d.name] = Tool{
			Name:        d.name,
			Description: d.desc,
			InputSchema: json.RawMessage(d.schema),
		}
	}
}
