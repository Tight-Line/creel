// Package server implements the Creel gRPC server.
package server

import (
	"fmt"
	"net"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"google.golang.org/grpc"

	"github.com/Tight-Line/creel/internal/auth"
)

// Server wraps a gRPC server with Creel services.
type Server struct {
	grpcServer *grpc.Server
	port       int
	// Registry is a dedicated Prometheus registry for gRPC metrics.
	// Using a custom registry avoids conflicts with the default metrics
	// that go-grpc-prometheus registers in init().
	Registry *prometheus.Registry
}

// New creates a new Server with auth and Prometheus interceptors.
func New(port int, apiKeyValidator *auth.APIKeyValidator, oidcValidator *auth.OIDCValidator) *Server {
	grpcMetrics := grpcprom.NewServerMetrics()
	grpcMetrics.EnableHandlingTimeHistogram()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcMetrics.UnaryServerInterceptor(),
			auth.UnaryInterceptor(apiKeyValidator, oidcValidator),
		),
	)
	grpcMetrics.InitializeMetrics(grpcServer)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(grpcMetrics)

	return &Server{
		grpcServer: grpcServer,
		port:       port,
		Registry:   reg,
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
