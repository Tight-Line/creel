// Package config handles loading and validating Creel server configuration.
package config

// Config is the top-level server configuration.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Auth          AuthConfig          `yaml:"auth"`
	Metadata      MetadataConfig      `yaml:"metadata"`
	VectorBackend VectorBackendConfig `yaml:"vector_backend"`
	Embedding     EmbeddingConfig     `yaml:"embedding"`
	Links         LinksConfig         `yaml:"links"`
	Compaction    CompactionConfig    `yaml:"compaction"`
}

type ServerConfig struct {
	GRPCPort    int `yaml:"grpc_port"`
	RESTPort    int `yaml:"rest_port"`
	MetricsPort int `yaml:"metrics_port"`
}

type AuthConfig struct {
	OIDCIssuer     string         `yaml:"oidc_issuer"`
	OIDCAudience   string         `yaml:"oidc_audience"`
	PrincipalClaim string         `yaml:"principal_claim"`
	APIKeys        []APIKeyConfig `yaml:"api_keys"`
}

type APIKeyConfig struct {
	Name    string `yaml:"name"`
	KeyHash string `yaml:"key_hash"`
}

type MetadataConfig struct {
	PostgresURL string `yaml:"postgres_url"`
}

type VectorBackendConfig struct {
	Type   string         `yaml:"type"`
	Config map[string]any `yaml:"config"`
}

type EmbeddingConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}

type LinksConfig struct {
	AutoLinkOnIngest   bool    `yaml:"auto_link_on_ingest"`
	AutoLinkThreshold  float64 `yaml:"auto_link_threshold"`
	MaxTraversalDepth  int     `yaml:"max_traversal_depth"`
}

type CompactionConfig struct {
	RetainCompactedChunks bool `yaml:"retain_compacted_chunks"`
}
