package config

import "testing"

func TestConfigStructInstantiation(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			GRPCPort:    8443,
			RESTPort:    8080,
			MetricsPort: 9090,
		},
		Auth: AuthConfig{
			PrincipalClaim: "sub",
			GroupsClaim:    "groups",
		},
		Metadata: MetadataConfig{
			PostgresURL: "postgres://localhost:5432/creel",
		},
		VectorBackend: VectorBackendConfig{
			Type:   "qdrant",
			Config: map[string]any{"url": "http://localhost:6334"},
		},
		Embedding: EmbeddingConfig{
			Provider: "openai",
			Model:    "text-embedding-3-small",
		},
		Links: LinksConfig{
			AutoLinkOnIngest:  true,
			AutoLinkThreshold: 0.85,
			MaxTraversalDepth: 3,
		},
		Compaction: CompactionConfig{
			RetainCompactedChunks: true,
		},
	}

	if cfg.Server.GRPCPort != 8443 {
		t.Errorf("expected GRPCPort 8443, got %d", cfg.Server.GRPCPort)
	}
	if cfg.VectorBackend.Type != "qdrant" {
		t.Errorf("expected vector backend type qdrant, got %s", cfg.VectorBackend.Type)
	}
}
