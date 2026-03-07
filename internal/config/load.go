package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file and applies environment variable overrides.
// Environment variables use the CREEL_ prefix with underscores replacing dots
// and nested keys joined by underscores. For example, CREEL_POSTGRES_HOST
// overrides postgres.host.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(&cfg)
	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for fields that were not specified.
func applyDefaults(cfg *Config) {
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = 8443
	}
	if cfg.Server.RESTPort == 0 {
		cfg.Server.RESTPort = 8080
	}
	if cfg.Server.MetricsPort == 0 {
		cfg.Server.MetricsPort = 9090
	}
	if cfg.Auth.PrincipalClaim == "" {
		cfg.Auth.PrincipalClaim = "sub"
	}
	if cfg.VectorBackend.Type == "" {
		cfg.VectorBackend.Type = "pgvector"
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.Schema == "" {
		cfg.Postgres.Schema = "creel"
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
}

// validate checks that all required fields are present and values are in range.
func validate(cfg *Config) error {
	if cfg.Postgres.Host == "" {
		return fmt.Errorf("postgres.host is required")
	}
	if cfg.Postgres.User == "" {
		return fmt.Errorf("postgres.user is required")
	}
	if cfg.Postgres.Name == "" {
		return fmt.Errorf("postgres.name is required")
	}
	if err := validatePort("server.grpc_port", cfg.Server.GRPCPort); err != nil {
		return err
	}
	if err := validatePort("server.rest_port", cfg.Server.RESTPort); err != nil {
		return err
	}
	if err := validatePort("server.metrics_port", cfg.Server.MetricsPort); err != nil {
		return err
	}
	return nil
}

func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", name, port)
	}
	return nil
}

// applyEnvOverrides reads CREEL_* environment variables and overrides
// matching config fields. It walks the Config struct via reflection,
// building the env key from yaml tags.
func applyEnvOverrides(cfg *Config) {
	applyEnvToStruct(reflect.ValueOf(cfg).Elem(), "CREEL")
}

func applyEnvToStruct(v reflect.Value, prefix string) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip yaml options like ",omitempty"
		tag = strings.Split(tag, ",")[0]
		envKey := prefix + "_" + strings.ToUpper(tag)

		if fv.Kind() == reflect.Struct {
			applyEnvToStruct(fv, envKey)
			continue
		}

		val, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			fv.SetString(val)
		case reflect.Int:
			var n int
			if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
				fv.SetInt(int64(n))
			}
		case reflect.Bool:
			fv.SetBool(val == "true" || val == "1")
		case reflect.Float64:
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				fv.SetFloat(f)
			}
		}
	}
}

// PostgresConfigFromEnv builds a PostgresConfig from CREEL_POSTGRES_* environment
// variables. Returns nil if CREEL_POSTGRES_HOST is not set. Used by integration tests.
func PostgresConfigFromEnv() *PostgresConfig {
	host := os.Getenv("CREEL_POSTGRES_HOST")
	if host == "" {
		return nil
	}

	cfg := &PostgresConfig{
		Host:     host,
		User:     os.Getenv("CREEL_POSTGRES_USER"),
		Password: os.Getenv("CREEL_POSTGRES_PASSWORD"),
		Name:     os.Getenv("CREEL_POSTGRES_NAME"),
		Schema:   os.Getenv("CREEL_POSTGRES_SCHEMA"),
		SSLMode:  os.Getenv("CREEL_POSTGRES_SSLMODE"),
	}

	var port int
	if _, err := fmt.Sscanf(os.Getenv("CREEL_POSTGRES_PORT"), "%d", &port); err == nil {
		cfg.Port = port
	}

	// Apply same defaults as Load.
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.Schema == "" {
		cfg.Schema = "creel"
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	return cfg
}
