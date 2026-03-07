package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tight-Line/creel/internal/config"
)

// TestOIDCValidator_WithFakeProvider tests OIDC validation with a fake JWKS server.
func TestOIDCValidator_WithFakeProvider(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	kid := "test-key-1"
	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                serverURL,
			"jwks_uri":                              serverURL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"kid": kid,
					"n":   b64url(privKey.N.Bytes()),
					"e":   b64url(big.NewInt(int64(privKey.E)).Bytes()),
				},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	providers := []config.OIDCProviderConfig{
		{Issuer: serverURL, Audience: "creel"},
	}

	validator, err := NewOIDCValidator(ctx, providers, "email", "groups")
	if err != nil {
		t.Fatalf("NewOIDCValidator: %v", err)
	}

	if !validator.HasProviders() {
		t.Error("expected HasProviders() = true")
	}

	t.Run("valid token", func(t *testing.T) {
		token := signJWT(t, privKey, kid, serverURL, "creel", map[string]any{
			"email":  "alice@example.com",
			"groups": []string{"engineering", "admin"},
		}, time.Now().Add(time.Hour))

		p, err := validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p == nil {
			t.Fatal("expected principal, got nil")
		}
		if p.ID != "user:alice@example.com" {
			t.Errorf("ID = %q, want user:alice@example.com", p.ID)
		}
		if len(p.Groups) != 2 {
			t.Errorf("Groups = %v, want 2 groups", p.Groups)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := signJWT(t, privKey, kid, serverURL, "creel", map[string]any{
			"email": "alice@example.com",
		}, time.Now().Add(-time.Hour))

		p, err := validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p != nil {
			t.Error("expected nil principal for expired token")
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		token := signJWT(t, privKey, kid, serverURL, "wrong-audience", map[string]any{
			"email": "alice@example.com",
		}, time.Now().Add(time.Hour))

		p, err := validator.Validate(ctx, token)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p != nil {
			t.Error("expected nil principal for wrong audience")
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		p, err := validator.Validate(ctx, "not.a.jwt")
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if p != nil {
			t.Error("expected nil principal for garbage token")
		}
	})
}

func TestNewOIDCValidator_NoProviders(t *testing.T) {
	v, err := NewOIDCValidator(context.Background(), nil, "sub", "groups")
	if err != nil {
		t.Fatalf("NewOIDCValidator: %v", err)
	}
	if v.HasProviders() {
		t.Error("expected HasProviders() = false")
	}
}

func TestClaimString(t *testing.T) {
	claims := map[string]any{
		"email": "alice@example.com",
		"num":   42,
	}

	// Happy path.
	s, ok := claimString(claims, "email")
	if !ok || s != "alice@example.com" {
		t.Errorf("claimString(email) = %q, %v", s, ok)
	}

	// Empty key.
	s, ok = claimString(claims, "")
	if ok || s != "" {
		t.Errorf("claimString('') = %q, %v; want empty/false", s, ok)
	}

	// Missing key.
	s, ok = claimString(claims, "missing")
	if ok || s != "" {
		t.Errorf("claimString(missing) = %q, %v", s, ok)
	}

	// Non-string value.
	_, ok = claimString(claims, "num")
	if ok {
		t.Errorf("claimString(num) ok = true for non-string")
	}
}

func TestClaimStringSlice(t *testing.T) {
	t.Run("array of strings", func(t *testing.T) {
		claims := map[string]any{
			"groups": []any{"eng", "admin"},
		}
		got := claimStringSlice(claims, "groups")
		if len(got) != 2 || got[0] != "group:eng" || got[1] != "group:admin" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("space-separated string", func(t *testing.T) {
		claims := map[string]any{
			"groups": "eng admin",
		}
		got := claimStringSlice(claims, "groups")
		if len(got) != 2 || got[0] != "group:eng" || got[1] != "group:admin" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		got := claimStringSlice(map[string]any{"groups": []any{"a"}}, "")
		if got != nil {
			t.Errorf("expected nil for empty key, got %v", got)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := claimStringSlice(map[string]any{}, "groups")
		if got != nil {
			t.Errorf("expected nil for missing key, got %v", got)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		claims := map[string]any{
			"groups": 42,
		}
		got := claimStringSlice(claims, "groups")
		if got != nil {
			t.Errorf("expected nil for int value, got %v", got)
		}
	})

	t.Run("array with non-string items", func(t *testing.T) {
		claims := map[string]any{
			"groups": []any{"eng", 42, "admin"},
		}
		got := claimStringSlice(claims, "groups")
		if len(got) != 2 {
			t.Errorf("expected 2 string items, got %v", got)
		}
	})
}

func TestOIDCValidator_EmptyPrincipalClaim(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	kid := "test-key-2"
	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                serverURL,
			"jwks_uri":                              serverURL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid,
				"n": b64url(privKey.N.Bytes()),
				"e": b64url(big.NewInt(int64(privKey.E)).Bytes()),
			}},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	providers := []config.OIDCProviderConfig{{Issuer: serverURL, Audience: "creel"}}

	// Empty principalClaim should default to "sub".
	validator, err := NewOIDCValidator(ctx, providers, "", "groups")
	if err != nil {
		t.Fatalf("NewOIDCValidator: %v", err)
	}

	token := signJWT(t, privKey, kid, serverURL, "creel", map[string]any{
		"sub": "alice",
	}, time.Now().Add(time.Hour))

	p, err := validator.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p == nil {
		t.Fatal("expected principal, got nil")
	}
	if p.ID != "user:alice" {
		t.Errorf("ID = %q, want user:alice", p.ID)
	}
}

func TestOIDCValidator_MissingPrincipalClaim(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	kid := "test-key-3"
	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                serverURL,
			"jwks_uri":                              serverURL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid,
				"n": b64url(privKey.N.Bytes()),
				"e": b64url(big.NewInt(int64(privKey.E)).Bytes()),
			}},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	providers := []config.OIDCProviderConfig{{Issuer: serverURL, Audience: "creel"}}

	// Use "email" as principal claim, but JWT will not have "email" field.
	validator, err := NewOIDCValidator(ctx, providers, "email", "groups")
	if err != nil {
		t.Fatalf("NewOIDCValidator: %v", err)
	}

	token := signJWT(t, privKey, kid, serverURL, "creel", map[string]any{
		"sub": "alice",
		// No "email" field.
	}, time.Now().Add(time.Hour))

	p, err := validator.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil principal for missing claim, got %v", p)
	}
}

func TestNewOIDCValidator_BadIssuer(t *testing.T) {
	providers := []config.OIDCProviderConfig{
		{Issuer: "http://localhost:99999", Audience: "creel"},
	}
	_, err := NewOIDCValidator(context.Background(), providers, "sub", "groups")
	if err == nil {
		t.Error("expected error for unreachable issuer")
	}
}

// signJWT creates a minimal RS256-signed JWT for testing.
func signJWT(t *testing.T, key *rsa.PrivateKey, kid, issuer, audience string, extra map[string]any, expiry time.Time) string {
	t.Helper()

	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	claims := map[string]any{
		"iss": issuer,
		"aud": audience,
		"exp": expiry.Unix(),
		"iat": time.Now().Unix(),
		"sub": "test-subject",
	}
	for k, v := range extra {
		claims[k] = v
	}
	payload, _ := json.Marshal(claims)

	input := b64url(header) + "." + b64url(payload)

	h := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}

	return input + "." + b64url(sig)
}

func b64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
