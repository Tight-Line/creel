package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	yaml := `
metadata:
  postgres_url: postgres://localhost:5432/creel
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
metadata:
  postgres_url: postgres://localhost/creel
`
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
}

func TestLoadEnvOverrides(t *testing.T) {
	yaml := `
metadata:
  postgres_url: postgres://localhost/creel
server:
  grpc_port: 8443
`
	path := writeTemp(t, yaml)

	t.Setenv("CREEL_METADATA_POSTGRES_URL", "postgres://override/creel")
	t.Setenv("CREEL_SERVER_GRPC_PORT", "7443")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Metadata.PostgresURL != "postgres://override/creel" {
		t.Errorf("PostgresURL = %q, want override", cfg.Metadata.PostgresURL)
	}
	if cfg.Server.GRPCPort != 7443 {
		t.Errorf("GRPCPort = %d, want 7443", cfg.Server.GRPCPort)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "missing postgres_url",
			yaml: `
server:
  grpc_port: 8443
`,
		},
		{
			name: "invalid grpc_port",
			yaml: `
metadata:
  postgres_url: postgres://localhost/creel
server:
  grpc_port: 99999
`,
		},
		{
			name: "zero port after override",
			yaml: `
metadata:
  postgres_url: postgres://localhost/creel
server:
  grpc_port: -1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.yaml)
			_, err := Load(path)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
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

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "creel.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
