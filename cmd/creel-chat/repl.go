package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// ensureTopic finds a topic by slug or creates it if missing. Returns the topic ID.
func ensureTopic(ctx context.Context, conn *grpc.ClientConn, slug string) (string, error) {
	client := pb.NewTopicServiceClient(conn)

	resp, err := client.ListTopics(ctx, &pb.ListTopicsRequest{})
	if err != nil {
		return "", fmt.Errorf("listing topics: %w", err)
	}

	for _, t := range resp.GetTopics() {
		if t.GetSlug() == slug {
			// Ensure we have a grant (idempotent; GrantAccess upserts).
			if _, err := client.GrantAccess(ctx, &pb.GrantAccessRequest{
				TopicId:    t.GetId(),
				Principal:  t.GetOwner(),
				Permission: pb.Permission_PERMISSION_ADMIN,
			}); err != nil {
				return "", fmt.Errorf("ensuring self access: %w", err)
			}
			return t.GetId(), nil
		}
	}

	topic, err := client.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug,
		Name: slug,
	})
	if err != nil {
		return "", fmt.Errorf("creating topic: %w", err)
	}

	// Grant ourselves admin so AccessibleTopics includes this topic in searches.
	// (AccessibleTopics only checks topic_grants, not ownership.)
	if _, err := client.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    topic.GetId(),
		Principal:  topic.GetOwner(),
		Permission: pb.Permission_PERMISSION_ADMIN,
	}); err != nil {
		return "", fmt.Errorf("granting self access: %w", err)
	}

	return topic.GetId(), nil
}

// resumeSession validates a document ID and returns the next safe sequence offset.
// Since there's no ListChunks RPC, we use the current unix timestamp as a high-water
// mark to avoid sequence collisions with previously ingested chunks.
func resumeSession(ctx context.Context, conn *grpc.ClientConn, docID string) (string, int32, error) {
	doc, err := pb.NewDocumentServiceClient(conn).GetDocument(ctx, &pb.GetDocumentRequest{
		Id: docID,
	})
	if err != nil {
		return "", 0, fmt.Errorf("document %s not found: %w", docID, err)
	}
	// Use a high offset based on time to avoid sequence collisions.
	offset := int32(time.Now().Unix() % 1_000_000_000)
	return doc.GetId(), offset, nil
}

// createSessionDoc creates a new document for this REPL session.
func createSessionDoc(ctx context.Context, conn *grpc.ClientConn, topicID string) (string, error) {
	slug := fmt.Sprintf("session-%d", time.Now().Unix())
	doc, err := pb.NewDocumentServiceClient(conn).CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topicID,
		Slug:    slug,
		Name:    slug,
		DocType: "chat-session",
	})
	if err != nil {
		return "", fmt.Errorf("creating document: %w", err)
	}
	return doc.GetId(), nil
}

// runLoop is the main REPL cycle: read, embed, search, prompt, call LLM, store.
// seqOffset is added to sequence numbers to avoid collisions when resuming a session.
func runLoop(ctx context.Context, conn *grpc.ClientConn, llm LLM, embedder Embedder, topicID, docID string, seqOffset int32) error {
	scanner := bufio.NewScanner(os.Stdin)
	var sessionMessages []ChatMessage
	var turn int32

	chunkClient := pb.NewChunkServiceClient(conn)
	retrievalClient := pb.NewRetrievalServiceClient(conn)

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			continue
		}

		turn++

		// Embed the user message.
		userEmbedding, err := embedder.Embed(ctx, userInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error embedding user message: %v\n", err)
			continue
		}

		// Search Creel for relevant past chunks.
		searchResp, err := retrievalClient.Search(ctx, &pb.SearchRequest{
			TopicIds:       []string{topicID},
			QueryEmbedding: userEmbedding,
			TopK:           topK,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error searching context: %v\n", err)
			continue
		}

		// Build LLM messages.
		messages := buildMessages(searchResp.GetResults(), sessionMessages, userInput)

		// Call LLM.
		assistantReply, err := llm.Chat(ctx, messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error calling LLM: %v\n", err)
			continue
		}

		fmt.Printf("assistant> %s\n", assistantReply)

		// Track in session buffer.
		sessionMessages = append(sessionMessages,
			ChatMessage{Role: "user", Content: userInput},
			ChatMessage{Role: "assistant", Content: assistantReply},
		)

		// Embed the assistant response.
		assistantEmbedding, err := embedder.Embed(ctx, assistantReply)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error embedding assistant message: %v\n", err)
			continue
		}

		// Ingest both chunks into Creel.
		userMeta, _ := structpb.NewStruct(map[string]any{
			"role": "user",
			"turn": float64(turn),
		})
		assistantMeta, _ := structpb.NewStruct(map[string]any{
			"role": "assistant",
			"turn": float64(turn),
		})

		seq := seqOffset + turn*2
		_, err = chunkClient.IngestChunks(ctx, &pb.IngestChunksRequest{
			DocumentId: docID,
			Chunks: []*pb.ChunkInput{
				{
					Content:   userInput,
					Embedding: userEmbedding,
					Sequence:  seq - 1,
					Metadata:  userMeta,
				},
				{
					Content:   assistantReply,
					Embedding: assistantEmbedding,
					Sequence:  seq,
					Metadata:  assistantMeta,
				},
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error ingesting chunks: %v\n", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	fmt.Println("")
	return nil
}

// buildMessages constructs the LLM prompt from retrieved context, session history, and current input.
func buildMessages(retrieved []*pb.SearchResult, session []ChatMessage, currentInput string) []ChatMessage {
	var messages []ChatMessage

	// System message with context instructions.
	systemPrompt := "You are a helpful assistant. Your conversation memory is stored in Creel and persists across sessions."

	// Add retrieved context if any.
	if len(retrieved) > 0 {
		// Sort by turn number (from metadata) for chronological ordering.
		sort.Slice(retrieved, func(i, j int) bool {
			ti := extractTurn(retrieved[i])
			tj := extractTurn(retrieved[j])
			return ti < tj
		})

		var contextParts []string
		for _, r := range retrieved {
			chunk := r.GetChunk()
			if chunk == nil {
				continue
			}
			role := extractRole(r)
			contextParts = append(contextParts, fmt.Sprintf("[%s]: %s", role, chunk.GetContent()))
		}
		if len(contextParts) > 0 {
			systemPrompt += "\n\nRelevant context from previous conversations:\n" + strings.Join(contextParts, "\n")
		}
	}

	messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})

	// Add recent session messages for continuity.
	messages = append(messages, session...)

	// Add current user input.
	messages = append(messages, ChatMessage{Role: "user", Content: currentInput})

	return messages
}

func extractTurn(r *pb.SearchResult) float64 {
	chunk := r.GetChunk()
	if chunk == nil || chunk.GetMetadata() == nil {
		return 0
	}
	if v, ok := chunk.GetMetadata().GetFields()["turn"]; ok {
		return v.GetNumberValue()
	}
	return 0
}

func extractRole(r *pb.SearchResult) string {
	chunk := r.GetChunk()
	if chunk == nil || chunk.GetMetadata() == nil {
		return "unknown"
	}
	if v, ok := chunk.GetMetadata().GetFields()["role"]; ok {
		return v.GetStringValue()
	}
	return "unknown"
}
