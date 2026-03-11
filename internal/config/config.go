// Package config handles loading and validating Creel server configuration.
package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// MaxGRPCMessageSize is the maximum gRPC message size (50 MB).
// Used by the server, CLI, chat, and MCP binaries.
const MaxGRPCMessageSize = 50 * 1024 * 1024

// GRPCEndpoint holds the parsed components of a gRPC endpoint URL.
type GRPCEndpoint struct {
	// Host is the host:port target for grpc.NewClient.
	Host string
	// TLS is true when the scheme is https.
	TLS bool
}

// ParseGRPCEndpoint parses a gRPC endpoint string. Accepted formats:
//
//   - https://host or https://host:port (TLS, default port 443)
//   - http://host:port (plaintext, port required)
//   - host:port (legacy format; plaintext)
func ParseGRPCEndpoint(raw string) (GRPCEndpoint, error) {
	// Legacy host:port format has no scheme. Detect it by checking for "://".
	if !strings.Contains(raw, "://") {
		return GRPCEndpoint{Host: raw, TLS: false}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return GRPCEndpoint{}, fmt.Errorf("invalid endpoint URL %q: %w", raw, err)
	}

	switch u.Scheme {
	case "https":
		host := u.Host
		if u.Port() == "" {
			host = u.Hostname() + ":443"
		}
		return GRPCEndpoint{Host: host, TLS: true}, nil
	case "http":
		if u.Port() == "" {
			return GRPCEndpoint{}, fmt.Errorf("http:// endpoint requires an explicit port (e.g. http://%s:8443)", u.Hostname())
		}
		return GRPCEndpoint{Host: u.Host, TLS: false}, nil
	default:
		return GRPCEndpoint{}, fmt.Errorf("unsupported scheme %q in endpoint URL %q (use http:// or https://)", u.Scheme, raw)
	}
}

// Config is the top-level server configuration.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Auth          AuthConfig          `yaml:"auth"`
	Postgres      PostgresConfig      `yaml:"postgres"`
	VectorBackend VectorBackendConfig `yaml:"vector_backend"`
	Embedding     EmbeddingConfig     `yaml:"embedding"`
	Links         LinksConfig         `yaml:"links"`
	Compaction    CompactionConfig    `yaml:"compaction"`
	EncryptionKey string              `yaml:"encryption_key"`
	Workers       WorkersConfig       `yaml:"workers"`
}

type WorkersConfig struct {
	Concurrency  int           `yaml:"concurrency"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

type ServerConfig struct {
	GRPCPort    int `yaml:"grpc_port"`
	RESTPort    int `yaml:"rest_port"`
	MetricsPort int `yaml:"metrics_port"`
}

type AuthConfig struct {
	Providers      []OIDCProviderConfig `yaml:"providers"`
	PrincipalClaim string               `yaml:"principal_claim"`
	GroupsClaim    string               `yaml:"groups_claim"`
	APIKeys        []APIKeyConfig       `yaml:"api_keys"`
}

// OIDCProviderConfig defines a trusted OIDC identity provider.
// Multiple providers can be configured to support different IdPs simultaneously.
type OIDCProviderConfig struct {
	Issuer   string `yaml:"issuer"`
	Audience string `yaml:"audience"`
}

type APIKeyConfig struct {
	Name      string `yaml:"name"`
	KeyHash   string `yaml:"key_hash"`
	Principal string `yaml:"principal"` // principal identity this key authenticates as
}

// PostgresConfig holds structured PostgreSQL connection parameters.
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Schema   string `yaml:"schema"`
	SSLMode  string `yaml:"sslmode"`
}

// URL returns the connection string with search_path set to the configured schema
// followed by public (so extensions installed in public are accessible).
func (p PostgresConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&search_path=%s,public",
		p.User, p.Password, p.Host, p.Port, p.Name, p.SSLMode, p.Schema,
	)
}

// BaseURL returns the connection string without search_path (for schema creation).
func (p PostgresConfig) BaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.Name, p.SSLMode,
	)
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
	AutoLinkOnIngest  bool    `yaml:"auto_link_on_ingest"`
	AutoLinkThreshold float64 `yaml:"auto_link_threshold"`
	MaxTraversalDepth int     `yaml:"max_traversal_depth"`
}

type CompactionConfig struct {
	RetainCompactedChunks bool `yaml:"retain_compacted_chunks"`
}
