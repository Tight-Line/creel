package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	apiKeyPrefix = "creel_ak_"
	keyBytes     = 32
)

// GenerateAPIKey creates a new API key with the creel_ak_ prefix.
// Returns the raw key (to show once) and its SHA-256 hash (to store).
func GenerateAPIKey() (raw string, hash string, prefix string, err error) {
	b := make([]byte, keyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generating random bytes: %w", err)
	}
	raw = apiKeyPrefix + hex.EncodeToString(b)
	hash = HashAPIKey(raw)
	prefix = raw[:len(apiKeyPrefix)+8]
	return raw, hash, prefix, nil
}

// HashAPIKey returns the SHA-256 hex digest of a raw API key.
func HashAPIKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// IsAPIKey returns true if the token looks like a Creel API key.
func IsAPIKey(token string) bool {
	return len(token) > len(apiKeyPrefix) && token[:len(apiKeyPrefix)] == apiKeyPrefix
}

// KeyLookup resolves an API key hash to a principal.
type KeyLookup interface {
	// LookupKeyHash returns the principal for the given key hash, or empty if not found.
	LookupKeyHash(ctx context.Context, hash string) (*Principal, error)
}

// APIKeyValidator validates API keys against static config and dynamic DB keys.
type APIKeyValidator struct {
	// staticKeys maps key hash to Principal for keys configured in the YAML file.
	staticKeys map[string]*Principal
	// dynamicLookup checks the database for system account keys.
	dynamicLookup KeyLookup
}

// NewAPIKeyValidator creates a validator with static keys and an optional dynamic lookup.
func NewAPIKeyValidator(staticKeys map[string]*Principal, dynamicLookup KeyLookup) *APIKeyValidator {
	if staticKeys == nil {
		staticKeys = make(map[string]*Principal)
	}
	return &APIKeyValidator{
		staticKeys:    staticKeys,
		dynamicLookup: dynamicLookup,
	}
}

// Validate checks an API key and returns the associated principal.
func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*Principal, error) {
	hash := HashAPIKey(rawKey)

	// Check static config keys first.
	if p, ok := v.staticKeys[hash]; ok {
		return p, nil
	}

	// Check dynamic DB keys.
	if v.dynamicLookup != nil {
		return v.dynamicLookup.LookupKeyHash(ctx, hash)
	}

	return nil, nil
}
