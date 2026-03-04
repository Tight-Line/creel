package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/store"
)

// AdminServer implements the AdminService gRPC service.
type AdminServer struct {
	pb.UnimplementedAdminServiceServer
	pool         *pgxpool.Pool
	accountStore *store.SystemAccountStore
	version      string
}

// NewAdminServer creates a new admin service.
func NewAdminServer(pool *pgxpool.Pool, accountStore *store.SystemAccountStore, version string) *AdminServer {
	return &AdminServer{
		pool:         pool,
		accountStore: accountStore,
		version:      version,
	}
}

// Health checks database connectivity and returns server status.
func (s *AdminServer) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	if err := s.pool.Ping(ctx); err != nil {
		return &pb.HealthResponse{
			Status:  "unhealthy",
			Version: s.version,
		}, nil
	}

	return &pb.HealthResponse{
		Status:  "ok",
		Version: s.version,
	}, nil
}

// CreateSystemAccount creates a new system account.
func (s *AdminServer) CreateSystemAccount(_ context.Context, _ *pb.CreateSystemAccountRequest) (*pb.CreateSystemAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in step 7")
}

// ListSystemAccounts lists all system accounts.
func (s *AdminServer) ListSystemAccounts(_ context.Context, _ *pb.ListSystemAccountsRequest) (*pb.ListSystemAccountsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in step 7")
}

// DeleteSystemAccount deletes a system account.
func (s *AdminServer) DeleteSystemAccount(_ context.Context, _ *pb.DeleteSystemAccountRequest) (*pb.DeleteSystemAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in step 7")
}

// RotateKey rotates the API key for a system account.
func (s *AdminServer) RotateKey(_ context.Context, _ *pb.RotateKeyRequest) (*pb.RotateKeyResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in step 7")
}

// RevokeKey revokes the API key for a system account.
func (s *AdminServer) RevokeKey(_ context.Context, _ *pb.RevokeKeyRequest) (*pb.RevokeKeyResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in step 7")
}
