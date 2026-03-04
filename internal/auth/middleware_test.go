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
