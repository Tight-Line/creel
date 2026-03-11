package server_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/server"
	"github.com/Tight-Line/creel/internal/store"
)

type memoryTestEnv struct {
	pool        *pgxpool.Pool
	conn        *grpc.ClientConn
	apiKey      string
	principal   string
	cleanup     func()
	memory      pb.MemoryServiceClient
	memoryStore *store.MemoryStore
}

func setupMemoryTestEnv(t *testing.T) *memoryTestEnv {
	t.Helper()

	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()

	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	pgURL := pgCfg.URL()
	if err := store.RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	// Clean test data.
	if _, err := pool.Exec(ctx, "DELETE FROM memories"); err != nil {
		pool.Close()
		t.Fatalf("cleaning memories: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM processing_jobs WHERE document_id IS NULL"); err != nil {
		pool.Close()
		t.Fatalf("cleaning docless jobs: %v", err)
	}

	memoryStore := store.NewMemoryStore(pool)
	jobStore := store.NewJobStore(pool)
	accountStore := store.NewSystemAccountStore(pool)

	acct, rawKey, err := accountStore.Create(ctx, "memory-test", "memory integration test")
	if err != nil {
		pool.Close()
		t.Fatalf("creating test system account: %v", err)
	}

	apiKeyValidator := auth.NewAPIKeyValidator(nil, accountStore)

	srv := server.New(0, apiKeyValidator, nil)
	memoryServer := server.NewMemoryServer(memoryStore, jobStore)
	pb.RegisterMemoryServiceServer(srv.GRPCServer(), memoryServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pool.Close()
		t.Fatalf("listening: %v", err)
	}

	go func() { _ = srv.GRPCServer().Serve(lis) }()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.GracefulStop()
		pool.Close()
		t.Fatalf("dialing: %v", err)
	}

	env := &memoryTestEnv{
		pool:      pool,
		conn:      conn,
		apiKey:    rawKey,
		principal: acct.Principal,
		cleanup: func() {
			_ = accountStore.Delete(ctx, acct.ID)
			_ = conn.Close()
			srv.GracefulStop()
			pool.Close()
		},
		memory:      pb.NewMemoryServiceClient(conn),
		memoryStore: memoryStore,
	}

	t.Cleanup(env.cleanup)
	return env
}

func (e *memoryTestEnv) authCtx() context.Context {
	md := metadata.Pairs("authorization", "Bearer "+e.apiKey)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// insertMemory creates a memory directly in the store, bypassing the RPC.
func (e *memoryTestEnv) insertMemory(t *testing.T, scope, content string) *store.Memory {
	t.Helper()
	m, err := e.memoryStore.Create(context.Background(), &store.Memory{
		Principal: e.principal,
		Scope:     scope,
		Content:   content,
	})
	if err != nil {
		t.Fatalf("inserting test memory: %v", err)
	}
	return m
}

func TestMemoryService_Integration_AddMemoryCreatesJob(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	resp, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   "test-scope",
		Content: "User prefers concise answers",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if resp.GetJobId() == "" {
		t.Fatal("expected non-empty job_id")
	}
}

func TestMemoryService_Integration_CRUD(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// Insert a memory directly for CRUD testing.
	mem := env.insertMemory(t, "test-scope", "User prefers concise answers")

	// GetMemory
	getResp, err := env.memory.GetMemory(ctx, &pb.GetMemoryRequest{Scope: "test-scope"})
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if len(getResp.GetMemories()) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(getResp.GetMemories()))
	}

	// Insert a second memory in a different scope.
	env.insertMemory(t, "work", "Work related memory")

	// ListScopes
	scopesResp, err := env.memory.ListScopes(ctx, &pb.ListScopesRequest{})
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}
	if len(scopesResp.GetScopes()) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopesResp.GetScopes()))
	}

	// ListMemories
	listResp, err := env.memory.ListMemories(ctx, &pb.ListMemoriesRequest{Scope: "test-scope"})
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(listResp.GetMemories()) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(listResp.GetMemories()))
	}

	// UpdateMemory
	updated, err := env.memory.UpdateMemory(ctx, &pb.UpdateMemoryRequest{
		Id:      mem.ID,
		Content: "Updated: user prefers verbose answers",
	})
	if err != nil {
		t.Fatalf("UpdateMemory: %v", err)
	}
	if updated.GetContent() != "Updated: user prefers verbose answers" {
		t.Fatalf("expected updated content, got %q", updated.GetContent())
	}

	// DeleteMemory (soft delete)
	_, err = env.memory.DeleteMemory(ctx, &pb.DeleteMemoryRequest{Id: mem.ID})
	if err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	// Verify deleted memory is not in active list.
	listResp, err = env.memory.ListMemories(ctx, &pb.ListMemoriesRequest{Scope: "test-scope"})
	if err != nil {
		t.Fatalf("ListMemories after delete: %v", err)
	}
	if len(listResp.GetMemories()) != 0 {
		t.Fatalf("expected 0 memories after delete, got %d", len(listResp.GetMemories()))
	}

	// Verify deleted memory is included with flag.
	listResp, err = env.memory.ListMemories(ctx, &pb.ListMemoriesRequest{
		Scope:              "test-scope",
		IncludeInvalidated: true,
	})
	if err != nil {
		t.Fatalf("ListMemories with invalidated: %v", err)
	}
	if len(listResp.GetMemories()) != 1 {
		t.Fatalf("expected 1 memory with invalidated, got %d", len(listResp.GetMemories()))
	}
	if listResp.GetMemories()[0].GetStatus() != "invalidated" {
		t.Fatalf("expected invalidated status, got %q", listResp.GetMemories()[0].GetStatus())
	}
}

func TestMemoryService_Integration_DefaultScope(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// AddMemory with no scope should create a job with "default" scope.
	resp, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Content: "Test memory",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if resp.GetJobId() == "" {
		t.Fatal("expected non-empty job_id")
	}
}

func TestMemoryService_Integration_SearchFallback(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// Insert memories directly.
	env.insertMemory(t, "search-test", "Memory one")
	env.insertMemory(t, "search-test", "Memory two")

	// Search without embedding should fall back to returning all memories.
	resp, err := env.memory.SearchMemories(ctx, &pb.SearchMemoriesRequest{
		Scope: "search-test",
	})
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(resp.GetResults()) != 2 {
		t.Fatalf("expected 2 results in fallback, got %d", len(resp.GetResults()))
	}
}

func TestMemoryService_Integration_Unauthenticated(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background() // No auth

	_, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Content: "should fail",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestMemoryService_Integration_AddWithTriple(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// AddMemory with triple fields should create a job (triple fields are
	// stored in job progress for the maintenance worker to process).
	resp, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:     "triple-test",
		Content:   "User likes fly fishing",
		Subject:   "user",
		Predicate: "likes",
		Object:    "fly fishing",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if resp.GetJobId() == "" {
		t.Fatal("expected non-empty job_id")
	}
}
