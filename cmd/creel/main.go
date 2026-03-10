package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/crypto"
	"github.com/Tight-Line/creel/internal/fetch"
	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/retrieval"
	"github.com/Tight-Line/creel/internal/server"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector/pgvector"
	"github.com/Tight-Line/creel/internal/worker"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgCfg := cfg.Postgres

	if *migrateOnly {
		if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
			return fmt.Errorf("ensuring schema: %w", err)
		}
		if err := store.RunMigrations(pgCfg.URL(), "migrations"); err != nil {
			return fmt.Errorf("running migrations: %w", err)
		}
		fmt.Println("creel: migrations up to date")
		return nil
	}

	// Open database pool.
	fullURL := pgCfg.URL()
	pool, err := store.NewPool(ctx, fullURL)
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
	linkStore := store.NewLinkStore(pool)
	vectorBackend := pgvector.New(pool)
	authorizer := auth.NewGrantAuthorizer(grantStore)

	// Create config stores (requires encryption key for API key configs).
	var encryptor *crypto.Encryptor
	var apiKeyConfigStore *store.APIKeyConfigStore
	if cfg.EncryptionKey != "" {
		encryptor, err = crypto.NewEncryptor(cfg.EncryptionKey)
		if err != nil {
			return fmt.Errorf("creating encryptor: %w", err)
		}
	}
	apiKeyConfigStore = store.NewAPIKeyConfigStore(pool, encryptor)
	llmConfigStore := store.NewLLMConfigStore(pool)
	embeddingConfigStore := store.NewEmbeddingConfigStore(pool)
	extractionPromptConfigStore := store.NewExtractionPromptConfigStore(pool)

	// Create job store and worker pool.
	jobStore := store.NewJobStore(pool)
	workerPool := worker.NewPool(jobStore, cfg.Workers.Concurrency, cfg.Workers.PollInterval, slog.Default())

	// Create fetcher and register workers.
	httpFetcher := fetch.NewHTTPFetcher()
	extractionWorker := worker.NewExtractionWorker(docStore, jobStore)
	workerPool.Register(extractionWorker)

	// TODO: replace stub LLM provider with a real one when available.
	llmProvider := llm.NewStubProvider(`{"facts": []}`)

	// Create and register chunking and embedding workers.
	chunkingWorker := worker.NewChunkingWorker(docStore, chunkStore, jobStore, topicStore, llmProvider)
	workerPool.Register(chunkingWorker)

	embeddingProvider := &dynamicEmbeddingProvider{
		embCfgStore:    embeddingConfigStore,
		apiKeyCfgStore: apiKeyConfigStore,
		fallbackDim:    vectorBackend.EmbeddingDimension(),
	}
	embeddingWorker := worker.NewEmbeddingWorker(docStore, chunkStore, topicStore, jobStore, vectorBackend, embeddingProvider)
	workerPool.Register(embeddingWorker)

	// Create and register memory workers.
	memoryStore := store.NewMemoryStore(pool)
	memExtractionWorker := worker.NewMemoryExtractionWorker(docStore, chunkStore, topicStore, jobStore, llmProvider)
	workerPool.Register(memExtractionWorker)
	memMaintenanceWorker := worker.NewMemoryMaintenanceWorker(memoryStore, jobStore, vectorBackend, embeddingProvider, llmProvider)
	workerPool.Register(memMaintenanceWorker)

	// Create and register compaction worker.
	compactionStore := store.NewCompactionStore(pool)
	compactionWorker := worker.NewCompactionWorker(chunkStore, linkStore, compactionStore, docStore, jobStore, vectorBackend, embeddingProvider, llmProvider)
	workerPool.Register(compactionWorker)

	// Create and wire server.
	srv := server.New(cfg.Server.GRPCPort, apiKeyValidator, oidcValidator)
	adminServer := server.NewAdminServer(pool, accountStore, version)
	topicServer := server.NewTopicServer(topicStore, authorizer, embeddingConfigStore)
	docServer := server.NewDocumentServer(docStore, jobStore, httpFetcher, authorizer)
	chunkServer := server.NewChunkServer(chunkStore, docStore, topicStore, jobStore, vectorBackend, authorizer)
	searcher := retrieval.NewSearcher(chunkStore, docStore, authorizer, vectorBackend)
	contextFetcher := retrieval.NewContextFetcher(chunkStore, authorizer)
	singleEmbedder := &singleTextEmbedder{batch: embeddingProvider}
	retrievalServer := server.NewRetrievalServer(searcher, contextFetcher, singleEmbedder)
	configServer := server.NewConfigServer(apiKeyConfigStore, llmConfigStore, embeddingConfigStore, extractionPromptConfigStore)
	jobServer := server.NewJobServer(jobStore, docStore, authorizer)
	linkServer := server.NewLinkServer(linkStore, chunkStore, authorizer)
	memoryServer := server.NewMemoryServer(memoryStore, vectorBackend, singleEmbedder)
	pb.RegisterAdminServiceServer(srv.GRPCServer(), adminServer)
	pb.RegisterTopicServiceServer(srv.GRPCServer(), topicServer)
	pb.RegisterDocumentServiceServer(srv.GRPCServer(), docServer)
	pb.RegisterChunkServiceServer(srv.GRPCServer(), chunkServer)
	pb.RegisterLinkServiceServer(srv.GRPCServer(), linkServer)
	pb.RegisterRetrievalServiceServer(srv.GRPCServer(), retrievalServer)
	pb.RegisterConfigServiceServer(srv.GRPCServer(), configServer)
	pb.RegisterJobServiceServer(srv.GRPCServer(), jobServer)
	pb.RegisterMemoryServiceServer(srv.GRPCServer(), memoryServer)
	compactionServer := server.NewCompactionServer(chunkStore, linkStore, compactionStore, docStore, jobStore, vectorBackend, authorizer)
	pb.RegisterCompactionServiceServer(srv.GRPCServer(), compactionServer)

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 2)

	// Start gRPC server.
	go func() {
		errCh <- srv.Run()
	}()

	// Start REST gateway.
	go func() {
		errCh <- runRESTGateway(ctx, cfg.Server.GRPCPort, cfg.Server.RESTPort)
	}()

	// Start worker pool.
	workerPool.Start(ctx)

	select {
	case sig := <-sigCh:
		fmt.Printf("\ncreel: received %v, shutting down...\n", sig)
		workerPool.Stop()
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		workerPool.Stop()
		return err
	}
}

// runRESTGateway starts an HTTP server that proxies to the gRPC server using grpc-gateway.
func runRESTGateway(ctx context.Context, grpcPort, restPort int) error {
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	endpoint := fmt.Sprintf("localhost:%d", grpcPort)

	// Register all service handlers.
	handlers := []func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error{
		pb.RegisterAdminServiceHandlerFromEndpoint,
		pb.RegisterTopicServiceHandlerFromEndpoint,
		pb.RegisterDocumentServiceHandlerFromEndpoint,
		pb.RegisterChunkServiceHandlerFromEndpoint,
		pb.RegisterRetrievalServiceHandlerFromEndpoint,
		pb.RegisterLinkServiceHandlerFromEndpoint,
		pb.RegisterCompactionServiceHandlerFromEndpoint,
		pb.RegisterConfigServiceHandlerFromEndpoint,
		pb.RegisterJobServiceHandlerFromEndpoint,
		pb.RegisterMemoryServiceHandlerFromEndpoint,
	}
	for _, h := range handlers {
		if err := h(ctx, mux, endpoint, opts); err != nil {
			return fmt.Errorf("registering REST handler: %w", err)
		}
	}

	fmt.Printf("creel: REST gateway listening on :%d\n", restPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", restPort), mux); err != nil { // coverage:ignore - requires port conflict to test
		return fmt.Errorf("REST gateway: %w", err)
	}
	return nil
}

// dynamicEmbeddingProvider resolves the default embedding config from the
// database on each call, so config changes take effect without a restart.
type dynamicEmbeddingProvider struct {
	embCfgStore    *store.EmbeddingConfigStore
	apiKeyCfgStore *store.APIKeyConfigStore
	fallbackDim    int
}

func (d *dynamicEmbeddingProvider) resolve(ctx context.Context) (worker.EmbeddingProvider, error) {
	embCfg, err := d.embCfgStore.GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading default embedding config: %w", err)
	}
	if embCfg == nil {
		return worker.NewStubEmbeddingProvider(d.fallbackDim), nil
	}

	apiKey, err := d.apiKeyCfgStore.GetDecrypted(ctx, embCfg.APIKeyConfigID)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key for embedding config %q: %w", embCfg.Name, err)
	}

	switch embCfg.Provider {
	case "openai":
		return worker.NewOpenAIEmbeddingProvider(string(apiKey), embCfg.Model, embCfg.Dimensions), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", embCfg.Provider)
	}
}

func (d *dynamicEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	provider, err := d.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return provider.Embed(ctx, texts)
}

func (d *dynamicEmbeddingProvider) Dimensions() int {
	ctx := context.Background()
	provider, err := d.resolve(ctx)
	if err != nil {
		return d.fallbackDim
	}
	return provider.Dimensions()
}

// singleTextEmbedder adapts a batch worker.EmbeddingProvider to the
// server.EmbeddingProvider interface (single text -> single vector).
type singleTextEmbedder struct {
	batch worker.EmbeddingProvider
}

func (s *singleTextEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vecs, err := s.batch.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embedding provider returned no vectors")
	}
	return vecs[0], nil
}
