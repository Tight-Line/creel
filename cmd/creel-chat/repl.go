package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	return topic.GetId(), nil
}

// resumeSession validates a document ID, fetches prior chunks via GetContext,
// populates sessionMessages, and returns a safe sequence offset.
func resumeSession(ctx context.Context, conn *grpc.ClientConn, docID string) (string, int32, []ChatMessage, error) {
	doc, err := pb.NewDocumentServiceClient(conn).GetDocument(ctx, &pb.GetDocumentRequest{
		Id: docID,
	})
	if err != nil {
		return "", 0, nil, fmt.Errorf("document %s not found: %w", docID, err)
	}

	// Fetch all prior chunks for this document in sequence order.
	retrievalClient := pb.NewRetrievalServiceClient(conn)
	resp, err := retrievalClient.GetContext(ctx, &pb.GetContextRequest{
		DocumentId: doc.GetId(),
	})
	if err != nil {
		return "", 0, nil, fmt.Errorf("fetching session context: %w", err)
	}

	// Populate session messages from chunks and find the max sequence.
	var messages []ChatMessage
	var maxSeq int32
	for _, chunk := range resp.GetChunks() {
		role := "user"
		if chunk.GetMetadata() != nil {
			if v, ok := chunk.GetMetadata().GetFields()["role"]; ok {
				role = v.GetStringValue()
			}
		}
		messages = append(messages, ChatMessage{Role: role, Content: chunk.GetContent()})
		if chunk.GetSequence() > maxSeq {
			maxSeq = chunk.GetSequence()
		}
	}

	// Offset new sequences past existing ones.
	seqOffset := maxSeq + 1
	return doc.GetId(), seqOffset, messages, nil
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

// loadMemories fetches memories for the current principal across the given scopes.
// If scopes is nil or empty, returns all memories for the principal.
func loadMemories(ctx context.Context, conn *grpc.ClientConn, scopes []string) []*pb.Memory {
	client := pb.NewMemoryServiceClient(conn)
	resp, err := client.GetMemories(ctx, &pb.GetMemoriesRequest{
		Scopes: scopes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load memories: %v\n", err)
		return nil
	}
	return resp.GetMemories()
}

// handleUpload processes the /upload command.
func handleUpload(ctx context.Context, conn *grpc.ClientConn, topicID, filePath string) { // coverage:ignore - interactive REPL command that requires real gRPC server and filesystem
	if filePath == "" {
		fmt.Fprintln(os.Stderr, "usage: /upload <filepath>")
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		return
	}

	name := filepath.Base(filePath)
	client := pb.NewDocumentServiceClient(conn)
	resp, err := client.UploadDocument(ctx, &pb.UploadDocumentRequest{
		TopicId: topicID,
		Name:    name,
		File:    data,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error uploading document: %v\n", err)
		return
	}

	fmt.Printf("Document uploaded. ID: %s, Job ID: %s\n", resp.GetDocument().GetId(), resp.GetJobId())
	fmt.Println("Processing will complete in the background.")
}

// handleRemember processes the /remember command.
func handleRemember(ctx context.Context, conn *grpc.ClientConn, scope, text string) { // coverage:ignore - interactive REPL command that requires real gRPC server
	if text == "" {
		fmt.Fprintln(os.Stderr, "usage: /remember <text>")
		return
	}

	client := pb.NewMemoryServiceClient(conn)
	resp, err := client.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   scope,
		Content: text,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error adding memory: %v\n", err)
		return
	}
	fmt.Printf("Memory queued for processing (job: %s)\n", resp.GetJobId())
}

// handleForget processes the /forget command.
func handleForget(ctx context.Context, conn *grpc.ClientConn, _ Embedder, scope, text string) { // coverage:ignore - interactive REPL command that requires real gRPC server
	if text == "" {
		fmt.Fprintln(os.Stderr, "usage: /forget <text>")
		return
	}

	client := pb.NewMemoryServiceClient(conn)

	// Get all memories in the scope and find the best match by substring.
	getResp, err := client.GetMemories(ctx, &pb.GetMemoriesRequest{
		Scopes: []string{scope},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting memories: %v\n", err)
		return
	}

	lowerText := strings.ToLower(text)
	var best *pb.Memory
	for _, m := range getResp.GetMemories() {
		if strings.Contains(strings.ToLower(m.GetContent()), lowerText) {
			best = m
			break
		}
	}

	if best == nil {
		fmt.Println("No matching memory found.")
		return
	}

	_, err = client.DeleteMemory(ctx, &pb.DeleteMemoryRequest{
		Id: best.GetId(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting memory: %v\n", err)
		return
	}
	fmt.Printf("Forgot: %s\n", best.GetContent())
}

// runLoop is the main REPL cycle: read, embed, search, prompt, call LLM, store.
// seqOffset is added to sequence numbers to avoid collisions when resuming a session.
// priorMessages contains any messages loaded from a resumed session.
func runLoop(ctx context.Context, conn *grpc.ClientConn, llm LLM, embedder Embedder, topicID, docID string, seqOffset int32, priorMessages []ChatMessage) error {
	scanner := bufio.NewScanner(os.Stdin)
	sessionMessages := priorMessages
	var turn int32

	chunkClient := pb.NewChunkServiceClient(conn)
	retrievalClient := pb.NewRetrievalServiceClient(conn)
	memoryClient := pb.NewMemoryServiceClient(conn)

	// Resolve which scopes to read from. Default to the write scope.
	readScopes := memoryReadScopes
	if len(readScopes) == 0 {
		readScopes = []string{memoryScope}
	}

	// Load memories at session start.
	memories := loadMemories(ctx, conn, readScopes)

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			continue
		}

		// Check for slash commands.
		cmd := ParseCommand(userInput)
		switch cmd.Type {
		case CmdUpload:
			handleUpload(ctx, conn, topicID, cmd.Arg)
			continue
		case CmdRemember:
			handleRemember(ctx, conn, memoryScope, cmd.Arg)
			// Reload memories after adding.
			memories = loadMemories(ctx, conn, readScopes)
			continue
		case CmdForget:
			handleForget(ctx, conn, embedder, memoryScope, cmd.Arg)
			// Reload memories after deleting.
			memories = loadMemories(ctx, conn, readScopes)
			continue
		}

		turn++

		// Embed the user message.
		userEmbedding, err := embedder.Embed(ctx, userInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error embedding user message: %v\n", err)
			continue
		}

		// Search Creel for relevant past chunks, excluding the current session
		// document so RAG results come from other sessions only.
		searchReq := &pb.SearchRequest{
			QueryEmbedding:     userEmbedding,
			TopK:               topK,
			ExcludeDocumentIds: []string{docID},
		}
		if crossTopic {
			// Omit topic_ids to search across all accessible topics.
		} else {
			searchReq.TopicIds = []string{topicID}
		}

		searchResp, err := retrievalClient.Search(ctx, searchReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error searching context: %v\n", err)
			continue
		}

		// Build LLM messages.
		messages := buildMessages(searchResp.GetResults(), sessionMessages, userInput, memories)

		// Call LLM with streaming.
		fmt.Print("assistant> ")
		tokens, err := llm.ChatStream(ctx, messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nerror calling LLM: %v\n", err)
			continue
		}

		assistantReply, err := CollectStream(tokens, func(text string) {
			fmt.Print(text)
		})
		fmt.Println()

		if err != nil {
			fmt.Fprintf(os.Stderr, "error streaming LLM response: %v\n", err)
			continue
		}

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

		// Send this turn's messages for automatic memory extraction.
		_, err = memoryClient.AddMessages(ctx, &pb.AddMessagesRequest{
			Scope: memoryScope,
			Messages: []*pb.ConversationMessage{
				{Role: "user", Content: userInput},
				{Role: "assistant", Content: assistantReply},
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not send messages for memory extraction: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	fmt.Println("")
	return nil
}

// buildMessages constructs the LLM prompt from retrieved context, session history,
// current input, and per-principal memories.
func buildMessages(retrieved []*pb.SearchResult, session []ChatMessage, currentInput string, memories []*pb.Memory) []ChatMessage {
	var messages []ChatMessage

	// Build the system prompt with explicit structure explanation.
	var sb strings.Builder
	sb.WriteString("You are a helpful assistant. Your conversation memory is stored in Creel and persists across sessions.\n\n")
	sb.WriteString("Your context has two layers:\n")
	sb.WriteString("1. SESSION HISTORY: The user/assistant message sequence that follows this system message is the complete, verbatim record of this conversation session. It is authoritative. If asked to recall or replay the conversation, use it exactly.\n")
	sb.WriteString("2. RAG CONTEXT: Semantically retrieved snippets from OTHER conversation sessions (not the current one). These may provide useful background but are not part of the current conversation.\n")

	// Add memory section if available.
	memorySection := FormatMemoryPrompt(memories)
	if memorySection != "" {
		sb.WriteString(memorySection)
	}

	// Add retrieved context if any, with citation information.
	if len(retrieved) > 0 {
		sort.Slice(retrieved, func(i, j int) bool {
			return extractTurn(retrieved[i]) < extractTurn(retrieved[j])
		})

		var contextParts []string
		for _, r := range retrieved {
			chunk := r.GetChunk()
			if chunk == nil {
				continue
			}
			contextParts = append(contextParts, FormatCitedResult(r))
		}
		if len(contextParts) > 0 {
			sb.WriteString("\n--- RAG CONTEXT (from other sessions) ---\n")
			sb.WriteString(strings.Join(contextParts, "\n"))
			sb.WriteString("\n--- END RAG CONTEXT ---")
		}
	}

	messages = append(messages, ChatMessage{Role: "system", Content: sb.String()})

	// Session history: complete, ordered conversation record.
	messages = append(messages, session...)

	// Current user input.
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
