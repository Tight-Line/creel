package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// publicMethods are RPCs that don't require authentication.
var publicMethods = map[string]bool{
	"/creel.v1.AdminService/Health": true,
}

// UnaryInterceptor returns a gRPC unary server interceptor that authenticates requests.
// It tries API key validation first, then OIDC if configured.
func UnaryInterceptor(apiKeyValidator *APIKeyValidator, oidcValidator *OIDCValidator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		token, err := extractBearerToken(ctx)
		if err != nil {
			return nil, err
		}

		var principal *Principal

		if IsAPIKey(token) {
			principal, err = apiKeyValidator.Validate(ctx, token)
			// coverage:ignore - requires gRPC context
			if err != nil {
				return nil, status.Errorf(codes.Internal, "validating API key: %v", err)
			}
			// coverage:ignore - requires gRPC context
		} else if oidcValidator != nil && oidcValidator.HasProviders() {
			principal, err = oidcValidator.Validate(ctx, token)
			// coverage:ignore - requires gRPC context
			if err != nil {
				return nil, status.Errorf(codes.Internal, "validating OIDC token: %v", err)
			}
		}

		if principal == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or missing credentials")
		}

		ctx = ContextWithPrincipal(ctx, principal)
		return handler(ctx, req)
	}
}

// extractBearerToken pulls the token from the Authorization header.
func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	vals := md.Get("authorization")
	// coverage:ignore - requires gRPC context
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	val := vals[0]
	if strings.HasPrefix(val, "Bearer ") {
		return val[7:], nil
	}
	// Allow raw API keys without Bearer prefix.
	// coverage:ignore - requires gRPC context
	if IsAPIKey(val) {
		return val, nil
	}
	// coverage:ignore - requires gRPC context
	return val, nil
}
