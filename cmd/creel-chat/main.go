package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	endpoint      string
	apiKey        string
	useTLS        bool
	provider      string
	model         string
	embedProvider string
	embedModel    string
	ollamaURL     string
	topicSlug     string
	topK          int32
	resumeDocID   string
)

func main() {
	root := &cobra.Command{
		Use:   "creel-chat",
		Short: "Interactive REPL that uses Creel as conversation memory",
		RunE:  run,
	}

	root.Flags().StringVar(&endpoint, "endpoint", envOr("CREEL_ENDPOINT", "localhost:8443"), "Creel gRPC endpoint")
	root.Flags().StringVar(&apiKey, "api-key", os.Getenv("CREEL_API_KEY"), "Creel API key")
	root.Flags().BoolVar(&useTLS, "tls", false, "use TLS for gRPC connection")
	root.Flags().StringVar(&provider, "provider", "anthropic", "chat LLM provider (anthropic or openai)")
	root.Flags().StringVar(&model, "model", "", "override LLM model name")
	root.Flags().StringVar(&embedProvider, "embed-provider", "openai", "embedding provider (openai or ollama)")
	root.Flags().StringVar(&embedModel, "embed-model", "text-embedding-3-small", "embedding model name")
	root.Flags().StringVar(&ollamaURL, "ollama-url", "http://localhost:11434", "Ollama API base URL")
	root.Flags().StringVar(&topicSlug, "topic", "creel-chat", "topic slug for conversation storage")
	root.Flags().Int32Var(&topK, "top-k", 5, "number of context chunks to retrieve")
	root.Flags().StringVar(&resumeDocID, "resume", "", "resume a previous session by document ID")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required (--api-key or CREEL_API_KEY)")
	}

	conn, err := dial()
	if err != nil {
		return fmt.Errorf("connecting to Creel: %w", err)
	}
	defer conn.Close()

	embedder, err := newEmbedder(embedProvider, embedModel, ollamaURL)
	if err != nil {
		return err
	}

	llm, err := newLLM(provider, model)
	if err != nil {
		return err
	}

	ctx := authCtx()

	topicID, err := ensureTopic(ctx, conn, topicSlug)
	if err != nil {
		return fmt.Errorf("ensuring topic: %w", err)
	}

	var docID string
	var seqOffset int32
	if resumeDocID != "" {
		docID, seqOffset, err = resumeSession(ctx, conn, resumeDocID)
		if err != nil {
			return fmt.Errorf("resuming session: %w", err)
		}
		fmt.Printf("creel-chat: resumed session %s (topic: %s, endpoint: %s)\n", docID, topicSlug, endpoint)
	} else {
		docID, err = createSessionDoc(ctx, conn, topicID)
		if err != nil {
			return fmt.Errorf("creating session document: %w", err)
		}
		fmt.Printf("creel-chat: new session %s (topic: %s, endpoint: %s)\n", docID, topicSlug, endpoint)
	}

	err = runLoop(ctx, conn, llm, embedder, topicID, docID, seqOffset)

	// Print resume command on exit.
	fmt.Printf("\nTo resume this session:\n  creel-chat --resume %s --topic %s", docID, topicSlug)
	if endpoint != "localhost:8443" {
		fmt.Printf(" --endpoint %s", endpoint)
	}
	if useTLS {
		fmt.Print(" --tls")
	}
	fmt.Println()

	return err
}

func dial() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.NewClient(endpoint, opts...)
}

func authCtx() context.Context {
	ctx := context.Background()
	if apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)
	}
	return ctx
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
