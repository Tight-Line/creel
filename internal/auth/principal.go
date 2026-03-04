// Package auth provides authentication and authorization for Creel.
package auth

import "context"

// Principal represents an authenticated identity.
type Principal struct {
	// ID is the unique identifier (e.g. "user:alice@example.com" or "system:bootstrap").
	ID string
	// Groups are the group memberships (e.g. ["group:engineering"]).
	Groups []string
	// IsSystem is true for system accounts authenticated via API key.
	IsSystem bool
}

type principalKey struct{}

// ContextWithPrincipal returns a new context carrying the given principal.
func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext extracts the principal from the context, or nil if absent.
func PrincipalFromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}
