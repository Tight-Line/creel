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
	"github.com/Tight-Line/creel/internal/vector/pgvector"
)

type memoryTestEnv struct {
	pool    *pgxpool.Pool
	conn    *grpc.ClientConn
	apiKey  string
	cleanup func()
	memory  pb.MemoryServiceClient
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

	// Clean memories table.
	if _, err := pool.Exec(ctx, "DELETE FROM memories"); err != nil {
		pool.Close()
		t.Fatalf("cleaning memories: %v", err)
	}

	memoryStore := store.NewMemoryStore(pool)
	backend := pgvector.New(pool)
	accountStore := store.NewSystemAccountStore(pool)

	acct, rawKey, err := accountStore.Create(ctx, "memory-test", "memory integration test")
	if err != nil {
		pool.Close()
		t.Fatalf("creating test system account: %v", err)
	}

	apiKeyValidator := auth.NewAPIKeyValidator(nil, accountStore)

	srv := server.New(0, apiKeyValidator, nil)
	memoryServer := server.NewMemoryServer(memoryStore, backend, nil)
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
		pool:   pool,
		conn:   conn,
		apiKey: rawKey,
		cleanup: func() {
			_ = accountStore.Delete(ctx, acct.ID)
			_ = conn.Close()
			srv.GracefulStop()
			pool.Close()
		},
		memory: pb.NewMemoryServiceClient(conn),
	}

	t.Cleanup(env.cleanup)
	return env
}

func (e *memoryTestEnv) authCtx() context.Context {
	md := metadata.Pairs("authorization", "Bearer "+e.apiKey)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestMemoryService_Integration_CRUD(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// AddMemory
	mem, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   "test-scope",
		Content: "User prefers concise answers",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if mem.GetId() == "" {
		t.Fatal("expected non-empty ID")
	}
	if mem.GetStatus() != "active" {
		t.Fatalf("expected active status, got %q", mem.GetStatus())
	}
	if mem.GetScope() != "test-scope" {
		t.Fatalf("expected scope 'test-scope', got %q", mem.GetScope())
	}

	// GetMemory
	getResp, err := env.memory.GetMemory(ctx, &pb.GetMemoryRequest{Scope: "test-scope"})
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if len(getResp.GetMemories()) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(getResp.GetMemories()))
	}

	// Add a second memory in a different scope
	_, err = env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   "work",
		Content: "Work related memory",
	})
	if err != nil {
		t.Fatalf("AddMemory (second): %v", err)
	}

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
		Id:      mem.GetId(),
		Content: "Updated: user prefers verbose answers",
	})
	if err != nil {
		t.Fatalf("UpdateMemory: %v", err)
	}
	if updated.GetContent() != "Updated: user prefers verbose answers" {
		t.Fatalf("expected updated content, got %q", updated.GetContent())
	}

	// DeleteMemory (soft delete)
	_, err = env.memory.DeleteMemory(ctx, &pb.DeleteMemoryRequest{Id: mem.GetId()})
	if err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	// Verify deleted memory is not in active list
	listResp, err = env.memory.ListMemories(ctx, &pb.ListMemoriesRequest{Scope: "test-scope"})
	if err != nil {
		t.Fatalf("ListMemories after delete: %v", err)
	}
	if len(listResp.GetMemories()) != 0 {
		t.Fatalf("expected 0 memories after delete, got %d", len(listResp.GetMemories()))
	}

	// Verify deleted memory is included with flag
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

	// AddMemory with no scope should use "default"
	mem, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Content: "Test memory",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if mem.GetScope() != "default" {
		t.Fatalf("expected default scope, got %q", mem.GetScope())
	}
}

func TestMemoryService_Integration_SearchFallback(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := env.authCtx()

	// Add some memories.
	_, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   "search-test",
		Content: "Memory one",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	_, err = env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:   "search-test",
		Content: "Memory two",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}

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

	mem, err := env.memory.AddMemory(ctx, &pb.AddMemoryRequest{
		Scope:     "triple-test",
		Content:   "User likes fly fishing",
		Subject:   "user",
		Predicate: "likes",
		Object:    "fly fishing",
	})
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if mem.GetSubject() != "user" || mem.GetPredicate() != "likes" || mem.GetObject() != "fly fishing" {
		t.Fatalf("triple fields not preserved: %v", mem)
	}
}
