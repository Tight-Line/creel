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
func UnaryInterceptor(apiKeyValidator *APIKeyValidator) grpc.UnaryServerInterceptor {
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
			if err != nil {
				return nil, status.Errorf(codes.Internal, "validating API key: %v", err)
			}
		}
		// TODO: OIDC validation for non-API-key tokens (Step 4)

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
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	val := vals[0]
	if strings.HasPrefix(val, "Bearer ") {
		return val[7:], nil
	}
	// Allow raw API keys without Bearer prefix.
	if IsAPIKey(val) {
		return val, nil
	}
	return val, nil
}
