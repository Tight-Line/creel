// Package server implements the Creel gRPC server.
package server

import (
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/Tight-Line/creel/internal/auth"
)

// Server wraps a gRPC server with Creel services.
type Server struct {
	grpcServer *grpc.Server
	port       int
}

// New creates a new Server with auth interceptor and registers services.
// coverage:ignore - gRPC handler; tested via integration tests
func New(port int, apiKeyValidator *auth.APIKeyValidator, oidcValidator *auth.OIDCValidator) *Server {
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor(apiKeyValidator, oidcValidator)),
	)

	return &Server{
		grpcServer: grpcServer,
		port:       port,
	}
}

// GRPCServer returns the underlying gRPC server for registering services.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Run starts the gRPC server and blocks until it stops.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", s.port, err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	fmt.Printf("creel: gRPC server listening on :%d\n", s.port)
	return s.grpcServer.Serve(lis)
}

// GracefulStop gracefully shuts down the gRPC server.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}
