package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// ToolHandler executes MCP tool calls against Creel gRPC services.
type ToolHandler struct {
	apiKey    string
	topics    pb.TopicServiceClient
	docs      pb.DocumentServiceClient
	chunks    pb.ChunkServiceClient
	retrieval pb.RetrievalServiceClient
	memory    pb.MemoryServiceClient
	jobs      pb.JobServiceClient
}

// NewToolHandler creates a tool handler with gRPC service clients.
func NewToolHandler(
	apiKey string,
	topics pb.TopicServiceClient,
	docs pb.DocumentServiceClient,
	chunks pb.ChunkServiceClient,
	retrieval pb.RetrievalServiceClient,
	memory pb.MemoryServiceClient,
	jobs pb.JobServiceClient,
) *ToolHandler {
	return &ToolHandler{
		apiKey:    apiKey,
		topics:    topics,
		docs:      docs,
		chunks:    chunks,
		retrieval: retrieval,
		memory:    memory,
		jobs:      jobs,
	}
}

// Execute runs a tool by name with the given JSON arguments.
func (h *ToolHandler) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	ctx = h.authCtx(ctx)

	switch name {
	case "creel_search":
		return h.search(ctx, args)
	case "creel_get_context":
		return h.getContext(ctx, args)
	case "creel_add_memory":
		return h.addMemory(ctx, args)
	case "creel_search_memories":
		return h.searchMemories(ctx, args)
	case "creel_list_memories":
		return h.listMemories(ctx, args)
	case "creel_delete_memory":
		return h.deleteMemory(ctx, args)
	case "creel_upload_document":
		return h.uploadDocument(ctx, args)
	case "creel_list_topics":
		return h.listTopics(ctx)
	case "creel_get_document":
		return h.getDocument(ctx, args)
	case "creel_list_documents":
		return h.listDocuments(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (h *ToolHandler) authCtx(ctx context.Context) context.Context {
	if h.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+h.apiKey)
	}
	return ctx
}

func (h *ToolHandler) search(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		TopicIDs    []string `json:"topic_ids"`
		QueryText   string   `json:"query_text"`
		TopK        int32    `json:"top_k"`
		FollowLinks bool     `json:"follow_links"`
		LinkDepth   int32    `json:"link_depth"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}
	if p.TopK == 0 {
		p.TopK = 10
	}

	resp, err := h.retrieval.Search(ctx, &pb.SearchRequest{
		TopicIds:    p.TopicIDs,
		QueryText:   p.QueryText,
		TopK:        p.TopK,
		FollowLinks: p.FollowLinks,
		LinkDepth:   p.LinkDepth,
	})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) getContext(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		DocumentID string `json:"document_id"`
		LastN      int32  `json:"last_n"`
		Since      string `json:"since"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	req := &pb.GetContextRequest{
		DocumentId: p.DocumentID,
		LastN:      p.LastN,
	}
	if p.Since != "" {
		t, err := time.Parse(time.RFC3339, p.Since)
		if err != nil {
			return "", fmt.Errorf("parsing 'since' timestamp: %w", err)
		}
		req.Since = timestamppb.New(t)
	}
	resp, err := h.retrieval.GetContext(ctx, req)
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) addMemory(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Scope     string `json:"scope"`
		Content   string `json:"content"`
		Subject   string `json:"subject"`
		Predicate string `json:"predicate"`
		Object    string `json:"object"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	resp, err := h.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:     p.Scope,
		Content:   p.Content,
		Subject:   p.Subject,
		Predicate: p.Predicate,
		Object:    p.Object,
	})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) searchMemories(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		QueryText string `json:"query_text"`
		Scope     string `json:"scope"`
		TopK      int32  `json:"top_k"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}
	if p.TopK == 0 {
		p.TopK = 10
	}

	resp, err := h.memory.SearchMemories(ctx, &pb.SearchMemoriesRequest{
		Scope:     p.Scope,
		QueryText: p.QueryText,
		TopK:      p.TopK,
	})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) listMemories(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Scope              string `json:"scope"`
		IncludeInvalidated bool   `json:"include_invalidated"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	resp, err := h.memory.ListMemories(ctx, &pb.ListMemoriesRequest{
		Scope:              p.Scope,
		IncludeInvalidated: p.IncludeInvalidated,
	})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) deleteMemory(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	_, err := h.memory.DeleteMemory(ctx, &pb.DeleteMemoryRequest{Id: p.ID})
	if err != nil {
		return "", err
	}
	return `{"status": "deleted"}`, nil
}

func (h *ToolHandler) uploadDocument(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		TopicID     string `json:"topic_id"`
		Name        string `json:"name"`
		Content     string `json:"content"`
		Slug        string `json:"slug"`
		ContentType string `json:"content_type"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	ct := p.ContentType
	if ct == "" {
		ct = "text/plain"
	}

	meta, _ := structpb.NewStruct(nil)
	resp, err := h.docs.UploadDocument(ctx, &pb.UploadDocumentRequest{
		TopicId:     p.TopicID,
		Name:        p.Name,
		Slug:        p.Slug,
		File:        []byte(p.Content),
		ContentType: ct,
		Metadata:    meta,
	})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) listTopics(ctx context.Context) (string, error) {
	resp, err := h.topics.ListTopics(ctx, &pb.ListTopicsRequest{})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) getDocument(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	resp, err := h.docs.GetDocument(ctx, &pb.GetDocumentRequest{Id: p.ID})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func (h *ToolHandler) listDocuments(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		TopicID string `json:"topic_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parsing arguments: %w", err)
	}

	resp, err := h.docs.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: p.TopicID})
	if err != nil {
		return "", err
	}
	return marshalProto(resp)
}

func marshalProto(msg proto.Message) (string, error) {
	b, err := protojson.Marshal(msg)
	// coverage:ignore - protojson.Marshal only fails on invalid proto messages
	if err != nil {
		return "", fmt.Errorf("marshaling response: %w", err)
	}
	return string(b), nil
}
