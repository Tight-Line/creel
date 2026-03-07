package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/store"
)

// Pinger checks database connectivity. *pgxpool.Pool satisfies this interface.
type Pinger interface {
	Ping(ctx context.Context) error
}

// AdminServer implements the AdminService gRPC service.
type AdminServer struct {
	pb.UnimplementedAdminServiceServer
	pinger       Pinger
	accountStore *store.SystemAccountStore
	version      string
}

// NewAdminServer creates a new admin service.
func NewAdminServer(pinger Pinger, accountStore *store.SystemAccountStore, version string) *AdminServer {
	return &AdminServer{
		pinger:       pinger,
		accountStore: accountStore,
		version:      version,
	}
}

// Health checks database connectivity and returns server status.
func (s *AdminServer) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	if err := s.pinger.Ping(ctx); err != nil {
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

// CreateSystemAccount creates a new system account and returns its initial API key.
func (s *AdminServer) CreateSystemAccount(ctx context.Context, req *pb.CreateSystemAccountRequest) (*pb.CreateSystemAccountResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	acct, rawKey, err := s.accountStore.Create(ctx, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating system account: %v", err)
	}

	return &pb.CreateSystemAccountResponse{
		Account: storeAccountToProto(acct),
		ApiKey:  rawKey,
	}, nil
}

// ListSystemAccounts lists all system accounts.
func (s *AdminServer) ListSystemAccounts(ctx context.Context, _ *pb.ListSystemAccountsRequest) (*pb.ListSystemAccountsResponse, error) {
	accounts, err := s.accountStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing system accounts: %v", err)
	}

	pbAccounts := make([]*pb.SystemAccount, len(accounts))
	for i, a := range accounts {
		pbAccounts[i] = storeAccountToProto(&a)
	}

	return &pb.ListSystemAccountsResponse{Accounts: pbAccounts}, nil
}

// DeleteSystemAccount deletes a system account and its keys.
func (s *AdminServer) DeleteSystemAccount(ctx context.Context, req *pb.DeleteSystemAccountRequest) (*pb.DeleteSystemAccountResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.accountStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting system account: %v", err)
	}

	return &pb.DeleteSystemAccountResponse{}, nil
}

// RotateKey generates a new API key for the system account.
func (s *AdminServer) RotateKey(ctx context.Context, req *pb.RotateKeyRequest) (*pb.RotateKeyResponse, error) {
	if req.GetAccountId() == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}

	gracePeriod := time.Duration(req.GetGracePeriodSeconds()) * time.Second
	rawKey, err := s.accountStore.RotateKey(ctx, req.GetAccountId(), gracePeriod)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rotating key: %v", err)
	}

	return &pb.RotateKeyResponse{ApiKey: rawKey}, nil
}

// RevokeKey immediately revokes all active keys for the system account.
func (s *AdminServer) RevokeKey(ctx context.Context, req *pb.RevokeKeyRequest) (*pb.RevokeKeyResponse, error) {
	if req.GetAccountId() == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}

	if err := s.accountStore.RevokeKey(ctx, req.GetAccountId()); err != nil {
		return nil, status.Errorf(codes.Internal, "revoking key: %v", err)
	}

	return &pb.RevokeKeyResponse{}, nil
}

func storeAccountToProto(a *store.SystemAccount) *pb.SystemAccount {
	return &pb.SystemAccount{
		Id:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Principal:   a.Principal,
		CreatedAt:   timestamppb.New(a.CreatedAt),
	}
}
