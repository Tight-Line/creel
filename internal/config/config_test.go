package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadFromYAML(t *testing.T) {
	yaml := `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  grpc_port: 9443
auth:
  principal_claim: email
  providers:
    - issuer: https://accounts.google.com
      audience: creel
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.GRPCPort != 9443 {
		t.Errorf("GRPCPort = %d, want 9443", cfg.Server.GRPCPort)
	}
	if cfg.Server.RESTPort != 8080 {
		t.Errorf("RESTPort = %d, want default 8080", cfg.Server.RESTPort)
	}
	if cfg.Auth.PrincipalClaim != "email" {
		t.Errorf("PrincipalClaim = %q, want email", cfg.Auth.PrincipalClaim)
	}
	if len(cfg.Auth.Providers) != 1 {
		t.Fatalf("Providers len = %d, want 1", len(cfg.Auth.Providers))
	}
	if cfg.Auth.Providers[0].Issuer != "https://accounts.google.com" {
		t.Errorf("Issuer = %q", cfg.Auth.Providers[0].Issuer)
	}
}

func TestLoadDefaults(t *testing.T) {
	yaml := `
postgres:
  host: localhost
  user: creel
  name: creel
`
	// Clear env vars that CI sets so the default branches in applyDefaults fire.
	t.Setenv("CREEL_POSTGRES_PORT", "")
	t.Setenv("CREEL_POSTGRES_SCHEMA", "")
	t.Setenv("CREEL_POSTGRES_SSLMODE", "")

	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.GRPCPort != 8443 {
		t.Errorf("GRPCPort = %d, want default 8443", cfg.Server.GRPCPort)
	}
	if cfg.Auth.PrincipalClaim != "sub" {
		t.Errorf("PrincipalClaim = %q, want default sub", cfg.Auth.PrincipalClaim)
	}
	if cfg.VectorBackend.Type != "pgvector" {
		t.Errorf("VectorBackend.Type = %q, want default pgvector", cfg.VectorBackend.Type)
	}
	if cfg.Postgres.Port != 5432 {
		t.Errorf("Postgres.Port = %d, want default 5432", cfg.Postgres.Port)
	}
	if cfg.Postgres.Schema != "creel" {
		t.Errorf("Postgres.Schema = %q, want default creel", cfg.Postgres.Schema)
	}
	if cfg.Postgres.SSLMode != "disable" {
		t.Errorf("Postgres.SSLMode = %q, want default disable", cfg.Postgres.SSLMode)
	}
	if cfg.Workers.Concurrency != 4 {
		t.Errorf("Workers.Concurrency = %d, want default 4", cfg.Workers.Concurrency)
	}
	if cfg.Workers.PollInterval != 5*time.Second {
		t.Errorf("Workers.PollInterval = %v, want default 5s", cfg.Workers.PollInterval)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	yaml := `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  grpc_port: 8443
`
	path := writeTemp(t, yaml)

	t.Setenv("CREEL_POSTGRES_HOST", "override-host")
	t.Setenv("CREEL_SERVER_GRPC_PORT", "7443")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Postgres.Host != "override-host" {
		t.Errorf("Host = %q, want override-host", cfg.Postgres.Host)
	}
	if cfg.Server.GRPCPort != 7443 {
		t.Errorf("GRPCPort = %d, want 7443", cfg.Server.GRPCPort)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		env  map[string]string
	}{
		{
			name: "missing postgres host",
			yaml: `
postgres:
  user: creel
  name: creel
`,
			env: map[string]string{"CREEL_POSTGRES_HOST": ""},
		},
		{
			name: "missing postgres user",
			yaml: `
postgres:
  host: localhost
  name: creel
`,
			env: map[string]string{"CREEL_POSTGRES_USER": ""},
		},
		{
			name: "missing postgres name",
			yaml: `
postgres:
  host: localhost
  user: creel
`,
			env: map[string]string{"CREEL_POSTGRES_NAME": ""},
		},
		{
			name: "invalid grpc_port",
			yaml: `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  grpc_port: 99999
`,
		},
		{
			name: "zero port after override",
			yaml: `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  grpc_port: -1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env overrides so the missing field stays empty
			// even if outer environment provides a value.
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			path := writeTemp(t, tt.yaml)
			_, err := Load(path)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestLoadValidation_InvalidRESTPort(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  rest_port: 99999
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for invalid rest_port")
	}
}

func TestLoadValidation_InvalidMetricsPort(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  metrics_port: -1
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for invalid metrics_port")
	}
}

func TestLoadEnvOverrides_BoolAndFloat(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
links:
  auto_link_on_ingest: false
  auto_link_threshold: 0.5
`)
	t.Setenv("CREEL_LINKS_AUTO_LINK_ON_INGEST", "true")
	t.Setenv("CREEL_LINKS_AUTO_LINK_THRESHOLD", "0.95")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Links.AutoLinkOnIngest {
		t.Error("expected AutoLinkOnIngest = true after env override")
	}
	if cfg.Links.AutoLinkThreshold != 0.95 {
		t.Errorf("AutoLinkThreshold = %f, want 0.95", cfg.Links.AutoLinkThreshold)
	}
}

func TestLoadEnvOverrides_InvalidInt(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
server:
  grpc_port: 8443
`)
	t.Setenv("CREEL_SERVER_GRPC_PORT", "not_a_number")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Invalid int should leave the original value.
	if cfg.Server.GRPCPort != 8443 {
		t.Errorf("GRPCPort = %d, want 8443 (unchanged)", cfg.Server.GRPCPort)
	}
}

func TestLoadBadYAML(t *testing.T) {
	path := writeTemp(t, `{{{bad yaml`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected parse error, got nil")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected file error, got nil")
	}
}

func TestApplyEnvToStruct_SkipUntaggedFields(t *testing.T) {
	type inner struct {
		Tagged   string `yaml:"tagged"`
		Untagged string // no yaml tag
		Skipped  string `yaml:"-"`
	}
	v := inner{Tagged: "original", Untagged: "original", Skipped: "original"}
	t.Setenv("PREFIX_TAGGED", "overridden")
	t.Setenv("PREFIX_UNTAGGED", "should-not-change")
	t.Setenv("PREFIX_-", "should-not-change")
	applyEnvToStruct(reflect.ValueOf(&v).Elem(), "PREFIX")
	if v.Tagged != "overridden" {
		t.Errorf("Tagged = %q, want overridden", v.Tagged)
	}
	if v.Untagged != "original" {
		t.Errorf("Untagged = %q, want original (should be skipped)", v.Untagged)
	}
	if v.Skipped != "original" {
		t.Errorf("Skipped = %q, want original (yaml:- should be skipped)", v.Skipped)
	}
}

func TestPostgresConfig_URL(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "myhost",
		Port:     5433,
		User:     "myuser",
		Password: "mypass",
		Name:     "mydb",
		Schema:   "myschema",
		SSLMode:  "require",
	}
	url := cfg.URL()
	if !strings.Contains(url, "myhost:5433") {
		t.Errorf("URL missing host:port: %s", url)
	}
	if !strings.Contains(url, "search_path=myschema") {
		t.Errorf("URL missing search_path: %s", url)
	}
}

func TestPostgresConfig_BaseURL(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "myhost",
		Port:     5433,
		User:     "myuser",
		Password: "mypass",
		Name:     "mydb",
		Schema:   "myschema",
		SSLMode:  "require",
	}
	url := cfg.BaseURL()
	if strings.Contains(url, "search_path") {
		t.Errorf("BaseURL should not contain search_path: %s", url)
	}
}

func TestPostgresConfig_SchemaDefault(t *testing.T) {
	t.Setenv("CREEL_POSTGRES_SCHEMA", "")
	yaml := `
postgres:
  host: localhost
  user: creel
  name: creel
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Postgres.Schema != "creel" {
		t.Errorf("Schema = %q, want default creel", cfg.Postgres.Schema)
	}
}

func TestLoadEnvOverrides_Schema(t *testing.T) {
	yaml := `
postgres:
  host: localhost
  user: creel
  name: creel
`
	path := writeTemp(t, yaml)
	t.Setenv("CREEL_POSTGRES_SCHEMA", "custom_schema")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Postgres.Schema != "custom_schema" {
		t.Errorf("Schema = %q, want custom_schema", cfg.Postgres.Schema)
	}
}

func TestPostgresConfigFromEnv(t *testing.T) {
	// Without CREEL_POSTGRES_HOST, returns nil.
	t.Setenv("CREEL_POSTGRES_HOST", "")
	result := PostgresConfigFromEnv()
	if result != nil {
		t.Fatal("expected nil when CREEL_POSTGRES_HOST is empty")
	}

	t.Setenv("CREEL_POSTGRES_HOST", "testhost")
	t.Setenv("CREEL_POSTGRES_PORT", "5433")
	t.Setenv("CREEL_POSTGRES_USER", "testuser")
	t.Setenv("CREEL_POSTGRES_PASSWORD", "testpass")
	t.Setenv("CREEL_POSTGRES_NAME", "testdb")
	t.Setenv("CREEL_POSTGRES_SCHEMA", "testschema")
	t.Setenv("CREEL_POSTGRES_SSLMODE", "require")

	cfg := PostgresConfigFromEnv()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Host != "testhost" {
		t.Errorf("Host = %q", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.User != "testuser" {
		t.Errorf("User = %q", cfg.User)
	}
	if cfg.Schema != "testschema" {
		t.Errorf("Schema = %q", cfg.Schema)
	}
}

func TestPostgresConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("CREEL_POSTGRES_HOST", "testhost")
	// Clear other env vars so defaults are exercised.
	t.Setenv("CREEL_POSTGRES_PORT", "")
	t.Setenv("CREEL_POSTGRES_SCHEMA", "")
	t.Setenv("CREEL_POSTGRES_SSLMODE", "")

	cfg := PostgresConfigFromEnv()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Port != 5432 {
		t.Errorf("Port = %d, want default 5432", cfg.Port)
	}
	if cfg.Schema != "creel" {
		t.Errorf("Schema = %q, want default creel", cfg.Schema)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("SSLMode = %q, want default disable", cfg.SSLMode)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetting %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, orig)
		}
	})
}

func TestLoadValidation_EncryptionKeyBadLength(t *testing.T) {
	unsetEnv(t, "CREEL_ENCRYPTION_KEY")
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
encryption_key: "tooshort"
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for short encryption key")
	}
}

func TestLoadValidation_EncryptionKeyBadHex(t *testing.T) {
	unsetEnv(t, "CREEL_ENCRYPTION_KEY")
	// 64 chars but not valid hex (contains 'g').
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
encryption_key: "ghijklmnopqrstuv0123456789abcdef0123456789abcdef0123456789abcdef"
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for invalid hex encryption key")
	}
}

func TestLoadDefaults_Workers(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
`)
	t.Setenv("CREEL_WORKERS_CONCURRENCY", "")
	t.Setenv("CREEL_WORKERS_POLL_INTERVAL", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workers.Concurrency != 4 {
		t.Errorf("Workers.Concurrency = %d, want default 4", cfg.Workers.Concurrency)
	}
	if cfg.Workers.PollInterval != 5*time.Second {
		t.Errorf("Workers.PollInterval = %v, want default 5s", cfg.Workers.PollInterval)
	}
}

func TestLoadEnvOverrides_Workers(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
`)
	t.Setenv("CREEL_WORKERS_CONCURRENCY", "8")
	t.Setenv("CREEL_WORKERS_POLL_INTERVAL", "10s")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workers.Concurrency != 8 {
		t.Errorf("Workers.Concurrency = %d, want 8", cfg.Workers.Concurrency)
	}
	if cfg.Workers.PollInterval != 10*time.Second {
		t.Errorf("Workers.PollInterval = %v, want 10s", cfg.Workers.PollInterval)
	}
}

func TestLoadEnvOverrides_Duration_Invalid(t *testing.T) {
	path := writeTemp(t, `
postgres:
  host: localhost
  user: creel
  name: creel
`)
	t.Setenv("CREEL_WORKERS_POLL_INTERVAL", "not-a-duration")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Invalid duration should be ignored; defaults should apply.
	if cfg.Workers.PollInterval != 5*time.Second {
		t.Errorf("Workers.PollInterval = %v, want default 5s", cfg.Workers.PollInterval)
	}
}

func TestParseGRPCEndpoint(t *testing.T) {
	tests := []struct {
		input   string
		host    string
		tls     bool
		wantErr bool
	}{
		{"https://grpc.example.com", "grpc.example.com:443", true, false},
		{"https://grpc.example.com:8443", "grpc.example.com:8443", true, false},
		{"http://localhost", "", false, true},
		{"http://localhost:9090", "localhost:9090", false, false},
		{"127.0.0.1:8443", "127.0.0.1:8443", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ep, err := ParseGRPCEndpoint(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ep.Host != tt.host {
				t.Errorf("Host = %q, want %q", ep.Host, tt.host)
			}
			if ep.TLS != tt.tls {
				t.Errorf("TLS = %v, want %v", ep.TLS, tt.tls)
			}
		})
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "creel.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
