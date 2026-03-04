package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/store"
)

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

	// TODO: initialize stores, auth, gRPC server
	fmt.Println("creel: server not yet implemented")
	return nil
}
