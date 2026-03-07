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
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Run starts the gRPC server and blocks until it stops.
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", s.port, err)
	}

	fmt.Printf("creel: gRPC server listening on :%d\n", s.port)
	return s.grpcServer.Serve(lis)
}

// GracefulStop gracefully shuts down the gRPC server.
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}
