package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/mcp"
)

func main() {
	os.Exit(run())
}

func run() int {
	endpoint := envOr("CREEL_ENDPOINT", "127.0.0.1:8443")
	apiKey := os.Getenv("CREEL_API_KEY")
	useTLS := os.Getenv("CREEL_TLS") == "true"

	var opts []grpc.DialOption
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connecting to creel: %v\n", err)
		return 1
	}
	defer func() { _ = conn.Close() }()

	handler := mcp.NewToolHandler(
		apiKey,
		pb.NewTopicServiceClient(conn),
		pb.NewDocumentServiceClient(conn),
		pb.NewChunkServiceClient(conn),
		pb.NewRetrievalServiceClient(conn),
		pb.NewMemoryServiceClient(conn),
		pb.NewJobServiceClient(conn),
	)

	server := mcp.NewServer(handler)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := server.RunStdio(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
