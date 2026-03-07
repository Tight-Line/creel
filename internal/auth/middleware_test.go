package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
