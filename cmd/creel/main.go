package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/server"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector/pgvector"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config file (or set CREEL_CONFIG)")
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	flag.Parse()

	path := *configPath
	if path == "" {
		path = os.Getenv("CREEL_CONFIG")
	}
	if path == "" {
		path = "creel.yaml"
	}

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Printf("creel: config loaded (grpc_port=%d)\n", cfg.Server.GRPCPort)

	if err := store.RunMigrations(cfg.Metadata.PostgresURL, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	fmt.Println("creel: migrations up to date")

	if *migrateOnly {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Open database pool.
	pool, err := store.NewPool(ctx, cfg.Metadata.PostgresURL)
	if err != nil {
		return fmt.Errorf("opening database pool: %w", err)
	}
	defer pool.Close()

	// Set up auth.
	staticKeys := auth.StaticKeysFromConfig(cfg.Auth.APIKeys)
	accountStore := store.NewSystemAccountStore(pool)
	apiKeyValidator := auth.NewAPIKeyValidator(staticKeys, accountStore)

	var oidcValidator *auth.OIDCValidator
	if len(cfg.Auth.Providers) > 0 {
		oidcValidator, err = auth.NewOIDCValidator(ctx, cfg.Auth.Providers, cfg.Auth.PrincipalClaim, cfg.Auth.GroupsClaim)
		if err != nil {
			return fmt.Errorf("initializing OIDC: %w", err)
		}
	}

	// Create stores, vector backend, and authorizer.
	grantStore := store.NewGrantStore(pool)
	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	vectorBackend := pgvector.New(pool)
	authorizer := auth.NewGrantAuthorizer(grantStore)

	// Create and wire server.
	srv := server.New(cfg.Server.GRPCPort, apiKeyValidator, oidcValidator)
	adminServer := server.NewAdminServer(pool, accountStore, version)
	topicServer := server.NewTopicServer(topicStore, authorizer)
	docServer := server.NewDocumentServer(docStore, authorizer)
	chunkServer := server.NewChunkServer(chunkStore, docStore, vectorBackend, authorizer)
	pb.RegisterAdminServiceServer(srv.GRPCServer(), adminServer)
	pb.RegisterTopicServiceServer(srv.GRPCServer(), topicServer)
	pb.RegisterDocumentServiceServer(srv.GRPCServer(), docServer)
	pb.RegisterChunkServiceServer(srv.GRPCServer(), chunkServer)

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run()
	}()

	select {
	case sig := <-sigCh:
		fmt.Printf("\ncreel: received %v, shutting down...\n", sig)
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}
