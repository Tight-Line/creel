package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tight-Line/creel/internal/config"
)

func TestUnaryInterceptor_PublicMethod(t *testing.T) {
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), nil)

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/Health"}
	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler not called for public method")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
}

func TestUnaryInterceptor_ValidAPIKey(t *testing.T) {
	raw, hash, _, _ := GenerateAPIKey()
	staticKeys := map[string]*Principal{
		hash: {ID: "system:test", IsSystem: true},
	}
	interceptor := UnaryInterceptor(NewAPIKeyValidator(staticKeys, nil), nil)

	md := metadata.Pairs("authorization", "Bearer "+raw)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var gotPrincipal *Principal
	handler := func(ctx context.Context, req any) (any, error) {
		gotPrincipal = PrincipalFromContext(ctx)
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPrincipal == nil || gotPrincipal.ID != "system:test" {
		t.Errorf("principal = %v, want system:test", gotPrincipal)
	}
}

func TestUnaryInterceptor_MissingAuth(t *testing.T) {
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), nil)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(context.Background(), nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryInterceptor_MissingAuthHeader(t *testing.T) {
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), nil)

	// Metadata present but no authorization header.
	md := metadata.Pairs("other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryInterceptor_RawAPIKeyWithoutBearer(t *testing.T) {
	raw, hash, _, _ := GenerateAPIKey()
	staticKeys := map[string]*Principal{
		hash: {ID: "system:raw", IsSystem: true},
	}
	interceptor := UnaryInterceptor(NewAPIKeyValidator(staticKeys, nil), nil)

	// Send raw API key without "Bearer " prefix.
	md := metadata.Pairs("authorization", raw)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var gotPrincipal *Principal
	handler := func(ctx context.Context, req any) (any, error) {
		gotPrincipal = PrincipalFromContext(ctx)
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPrincipal == nil || gotPrincipal.ID != "system:raw" {
		t.Errorf("principal = %v, want system:raw", gotPrincipal)
	}
}

func TestUnaryInterceptor_NonBearerNonAPIKey(t *testing.T) {
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), nil)

	// Send a non-Bearer, non-API-key token.
	md := metadata.Pairs("authorization", "some-jwt-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryInterceptor_APIKeyValidationError(t *testing.T) {
	lookup := &errorKeyLookup{err: context.DeadlineExceeded}
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, lookup), nil)

	md := metadata.Pairs("authorization", "Bearer creel_ak_test1234567890abcdef")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("code = %v, want Internal", status.Code(err))
	}
}

type errorKeyLookup struct {
	err error
}

func (e *errorKeyLookup) LookupKeyHash(_ context.Context, _ string) (*Principal, error) {
	return nil, e.err
}

func TestUnaryInterceptor_OIDCValidation(t *testing.T) {
	// With no providers, a non-API-key Bearer token should be unauthenticated.
	oidcV, _ := NewOIDCValidator(context.Background(), nil, "sub", "groups")
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), oidcV)

	md := metadata.Pairs("authorization", "Bearer eyJhbGciOiJSUzI1NiJ9.fake.sig")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryInterceptor_OIDCDispatch_ValidToken(t *testing.T) {
	// Create a fake OIDC provider with signed JWTs to exercise the OIDC dispatch
	// branch (middleware.go:43) and the error check (middleware.go:45).
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	kid := "mw-test-key"
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

	oidcV, err := NewOIDCValidator(context.Background(), []config.OIDCProviderConfig{
		{Issuer: serverURL, Audience: "creel"},
	}, "email", "groups")
	if err != nil {
		t.Fatalf("NewOIDCValidator: %v", err)
	}

	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), oidcV)

	// Create a valid JWT that is NOT an API key, so the OIDC branch is taken.
	token := signJWT(t, privKey, kid, serverURL, "creel", map[string]any{
		"email":  "alice@example.com",
		"groups": []string{"eng"},
	}, time.Now().Add(time.Hour))

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var gotPrincipal *Principal
	handler := func(ctx context.Context, req any) (any, error) {
		gotPrincipal = PrincipalFromContext(ctx)
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.TopicService/ListTopics"}
	_, err = interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPrincipal == nil || gotPrincipal.ID != "user:alice@example.com" {
		t.Errorf("principal = %v, want user:alice@example.com", gotPrincipal)
	}
}

func TestUnaryInterceptor_InvalidKey(t *testing.T) {
	interceptor := UnaryInterceptor(NewAPIKeyValidator(nil, nil), nil)

	md := metadata.Pairs("authorization", "Bearer creel_ak_invalid")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/creel.v1.AdminService/CreateSystemAccount"}
	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}
