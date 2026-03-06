package auth

import (
	"github.com/Tight-Line/creel/internal/config"
)

// StaticKeysFromConfig builds the static key map from configured API keys.
// coverage:ignore - requires database
func StaticKeysFromConfig(keys []config.APIKeyConfig) map[string]*Principal {
	m := make(map[string]*Principal, len(keys))
	// coverage:ignore - requires database
	for _, k := range keys {
		m[k.KeyHash] = &Principal{
			ID:       k.Principal,
			IsSystem: true,
		}
	}
	// coverage:ignore - requires database
	return m
}
