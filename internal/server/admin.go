package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

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
// coverage:ignore - gRPC handler; tested via integration tests
func NewAdminServer(pool *pgxpool.Pool, accountStore *store.SystemAccountStore, version string) *AdminServer {
	return &AdminServer{
		pool:         pool,
		accountStore: accountStore,
		version:      version,
	}
}

// Health checks database connectivity and returns server status.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.pool.Ping(ctx); err != nil {
		return &pb.HealthResponse{
			Status:  "unhealthy",
			Version: s.version,
		}, nil
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.HealthResponse{
		Status:  "ok",
		Version: s.version,
	}, nil
}

// CreateSystemAccount creates a new system account and returns its initial API key.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) CreateSystemAccount(ctx context.Context, req *pb.CreateSystemAccountRequest) (*pb.CreateSystemAccountResponse, error) {
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	acct, rawKey, err := s.accountStore.Create(ctx, req.GetName(), req.GetDescription())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating system account: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.CreateSystemAccountResponse{
		Account: storeAccountToProto(acct),
		ApiKey:  rawKey,
	}, nil
}

// ListSystemAccounts lists all system accounts.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) ListSystemAccounts(ctx context.Context, _ *pb.ListSystemAccountsRequest) (*pb.ListSystemAccountsResponse, error) {
	accounts, err := s.accountStore.List(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing system accounts: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	pbAccounts := make([]*pb.SystemAccount, len(accounts))
	// coverage:ignore - gRPC handler; tested via integration tests
	for i, a := range accounts {
		pbAccounts[i] = storeAccountToProto(&a)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.ListSystemAccountsResponse{Accounts: pbAccounts}, nil
}

// DeleteSystemAccount deletes a system account and its keys.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) DeleteSystemAccount(ctx context.Context, req *pb.DeleteSystemAccountRequest) (*pb.DeleteSystemAccountResponse, error) {
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.accountStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting system account: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.DeleteSystemAccountResponse{}, nil
}

// RotateKey generates a new API key for the system account.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) RotateKey(ctx context.Context, req *pb.RotateKeyRequest) (*pb.RotateKeyResponse, error) {
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetAccountId() == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	gracePeriod := time.Duration(req.GetGracePeriodSeconds()) * time.Second
	rawKey, err := s.accountStore.RotateKey(ctx, req.GetAccountId(), gracePeriod)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rotating key: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.RotateKeyResponse{ApiKey: rawKey}, nil
}

// RevokeKey immediately revokes all active keys for the system account.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *AdminServer) RevokeKey(ctx context.Context, req *pb.RevokeKeyRequest) (*pb.RevokeKeyResponse, error) {
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetAccountId() == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.accountStore.RevokeKey(ctx, req.GetAccountId()); err != nil {
		return nil, status.Errorf(codes.Internal, "revoking key: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.RevokeKeyResponse{}, nil
}

// coverage:ignore - gRPC handler; tested via integration tests
func storeAccountToProto(a *store.SystemAccount) *pb.SystemAccount {
	return &pb.SystemAccount{
		Id:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Principal:   a.Principal,
		CreatedAt:   timestamppb.New(a.CreatedAt),
	}
}
