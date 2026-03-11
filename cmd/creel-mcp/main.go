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
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/mcp"
)

func main() {
	os.Exit(run())
}

func run() int {
	endpointURL := envOr("CREEL_GRPC_ENDPOINT", "http://127.0.0.1:8443")
	apiKey := os.Getenv("CREEL_API_KEY")
	verifyTLS := os.Getenv("CREEL_VERIFY_TLS") != "false"

	ep, err := config.ParseGRPCEndpoint(endpointURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid endpoint: %v\n", err)
		return 1
	}

	var opts []grpc.DialOption
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.MaxGRPCMessageSize),
			grpc.MaxCallSendMsgSize(config.MaxGRPCMessageSize),
		),
	)
	grpcAuthority := os.Getenv("CREEL_GRPC_AUTHORITY")
	if ep.TLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: !verifyTLS, //nolint:gosec // user-controlled flag for self-signed certs
		}
		if grpcAuthority != "" {
			tlsCfg.ServerName = grpcAuthority
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if grpcAuthority != "" {
		opts = append(opts, grpc.WithAuthority(grpcAuthority))
	}

	// passthrough:/// bypasses grpc-go's built-in DNS resolver, which does
	// TXT/SRV lookups that can hang on split-horizon DNS (e.g. macOS with
	// domain-specific resolvers). The OS resolver handles name resolution.
	conn, err := grpc.NewClient("passthrough:///"+ep.Host, opts...)
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
