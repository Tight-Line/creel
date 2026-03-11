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

	"github.com/Tight-Line/creel/internal/config"
)

var (
	endpointURL   string
	apiKey        string
	verifyTLS     bool
	authority     string
	grpcEP        config.GRPCEndpoint
	provider      string
	model         string
	embedProvider string
	embedModel    string
	ollamaURL     string
	topicSlug     string
	topK          int32
	resumeDocID   string
	memoryScope   string
	crossTopic    bool
)

func main() {
	root := &cobra.Command{
		Use:   "creel-chat",
		Short: "Interactive REPL that uses Creel as conversation memory",
		RunE:  run,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			var err error
			grpcEP, err = config.ParseGRPCEndpoint(endpointURL)
			return err
		},
	}

	root.Flags().StringVar(&endpointURL, "endpoint", envOr("CREEL_GRPC_ENDPOINT", "http://127.0.0.1:8443"), "gRPC endpoint URL (https://host or http://host:port)")
	root.Flags().StringVar(&apiKey, "api-key", os.Getenv("CREEL_API_KEY"), "Creel API key")
	root.Flags().BoolVar(&verifyTLS, "verify-tls", os.Getenv("CREEL_VERIFY_TLS") != "false", "verify TLS certificates (set CREEL_VERIFY_TLS=false to skip)")
	root.Flags().StringVar(&authority, "authority", os.Getenv("CREEL_GRPC_AUTHORITY"), "override the :authority header (for routing through proxies)")
	root.Flags().StringVar(&provider, "provider", "openai", "chat LLM provider (openai or anthropic)")
	root.Flags().StringVar(&model, "model", "", "override LLM model name")
	root.Flags().StringVar(&embedProvider, "embed-provider", "openai", "embedding provider (openai or ollama)")
	root.Flags().StringVar(&embedModel, "embed-model", "text-embedding-3-small", "embedding model name")
	root.Flags().StringVar(&ollamaURL, "ollama-url", "http://localhost:11434", "Ollama API base URL")
	root.Flags().StringVar(&topicSlug, "topic", "creel-chat", "topic slug for conversation storage")
	root.Flags().Int32Var(&topK, "top-k", 5, "number of context chunks to retrieve")
	root.Flags().StringVar(&resumeDocID, "resume", "", "resume a previous session by document ID")
	root.Flags().StringVar(&memoryScope, "memory-scope", "default", "memory scope for per-principal memories")
	root.Flags().BoolVar(&crossTopic, "cross-topic", false, "search across all accessible topics instead of just the current one")

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
	defer func() { _ = conn.Close() }()

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
	var priorMessages []ChatMessage
	if resumeDocID != "" {
		docID, seqOffset, priorMessages, err = resumeSession(ctx, conn, resumeDocID)
		if err != nil {
			return fmt.Errorf("resuming session: %w", err)
		}
		fmt.Printf("creel-chat: resumed session %s (%d prior messages, topic: %s, endpoint: %s)\n", docID, len(priorMessages), topicSlug, endpointURL)
	} else {
		docID, err = createSessionDoc(ctx, conn, topicID)
		if err != nil {
			return fmt.Errorf("creating session document: %w", err)
		}
		fmt.Printf("creel-chat: new session %s (topic: %s, endpoint: %s)\n", docID, topicSlug, endpointURL)
	}

	err = runLoop(ctx, conn, llm, embedder, topicID, docID, seqOffset, priorMessages)

	// Print resume command on exit.
	fmt.Printf("\nTo resume this session:\n  creel-chat --resume %s --topic %s", docID, topicSlug)
	if endpointURL != "http://127.0.0.1:8443" {
		fmt.Printf(" --endpoint %s", endpointURL)
	}
	fmt.Println()

	return err
}

func dial() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.MaxGRPCMessageSize),
			grpc.MaxCallSendMsgSize(config.MaxGRPCMessageSize),
		),
	)
	if grpcEP.TLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: !verifyTLS, //nolint:gosec // user-controlled flag for self-signed certs
		}
		if authority != "" {
			tlsCfg.ServerName = authority
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if authority != "" {
		opts = append(opts, grpc.WithAuthority(authority))
	}
	// passthrough:/// bypasses grpc-go's built-in DNS resolver, which does
	// TXT/SRV lookups that can hang on split-horizon DNS (e.g. macOS with
	// domain-specific resolvers). The OS resolver handles name resolution.
	return grpc.NewClient("passthrough:///"+grpcEP.Host, opts...)
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
