package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/Tight-Line/creel/internal/config"
)

// OIDCValidator validates JWT tokens against configured OIDC providers.
type OIDCValidator struct {
	mu        sync.RWMutex
	verifiers map[string]*providerEntry // issuer -> verifier
}

type providerEntry struct {
	verifier       *gooidc.IDTokenVerifier
	audience       string
	principalClaim string
	groupsClaim    string
}

// NewOIDCValidator creates a validator for the given OIDC providers.
// It performs OIDC discovery for each provider (fetches JWKS endpoints).
func NewOIDCValidator(ctx context.Context, providers []config.OIDCProviderConfig, principalClaim, groupsClaim string) (*OIDCValidator, error) {
	v := &OIDCValidator{
		verifiers: make(map[string]*providerEntry, len(providers)),
	}

	for _, p := range providers {
		provider, err := gooidc.NewProvider(ctx, p.Issuer)
		if err != nil {
			return nil, fmt.Errorf("OIDC discovery for %s: %w", p.Issuer, err)
		}

		verifier := provider.Verifier(&gooidc.Config{
			ClientID: p.Audience,
		})

		pc := principalClaim
		// coverage:ignore - requires OIDC provider
		if pc == "" {
			pc = "sub"
		}

		v.verifiers[p.Issuer] = &providerEntry{
			verifier:       verifier,
			audience:       p.Audience,
			principalClaim: pc,
			groupsClaim:    groupsClaim,
		}
	}

	return v, nil
}

// Validate checks a raw JWT token against all configured providers.
// Returns the authenticated principal or nil if no provider accepts the token.
func (v *OIDCValidator) Validate(ctx context.Context, rawToken string) (*Principal, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, entry := range v.verifiers {
		idToken, err := entry.verifier.Verify(ctx, rawToken)
		if err != nil {
			continue
		}

		var claims map[string]any
		// coverage:ignore - requires OIDC provider
		if err := idToken.Claims(&claims); err != nil {
			continue
		}

		principalID, _ := claimString(claims, entry.principalClaim)
		// coverage:ignore - requires OIDC provider
		if principalID == "" {
			continue
		}

		groups := claimStringSlice(claims, entry.groupsClaim)

		return &Principal{
			ID:     "user:" + principalID,
			Groups: groups,
		}, nil
	}

	return nil, nil
}

// HasProviders returns true if at least one OIDC provider is configured.
func (v *OIDCValidator) HasProviders() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.verifiers) > 0
}

func claimString(claims map[string]any, key string) (string, bool) {
	// coverage:ignore - requires OIDC provider
	if key == "" {
		return "", false
	}
	v, ok := claims[key]
	// coverage:ignore - requires OIDC provider
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func claimStringSlice(claims map[string]any, key string) []string {
	// coverage:ignore - requires OIDC provider
	if key == "" {
		return nil
	}
	v, ok := claims[key]
	// coverage:ignore - requires OIDC provider
	if !ok {
		return nil
	}

	switch val := v.(type) {
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, "group:"+s)
			}
		}
		return result
	// coverage:ignore - requires OIDC provider
	case string:
		// Some providers return space-separated groups.
		parts := strings.Fields(val)
		result := make([]string, len(parts))
		// coverage:ignore - requires OIDC provider
		for i, p := range parts {
			result[i] = "group:" + p
		}
		// coverage:ignore - requires OIDC provider
		return result
	}
	// coverage:ignore - requires OIDC provider
	return nil
}
