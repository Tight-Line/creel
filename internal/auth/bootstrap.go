package auth

import (
	"github.com/Tight-Line/creel/internal/config"
)

// StaticKeysFromConfig builds the static key map from configured API keys.
func StaticKeysFromConfig(keys []config.APIKeyConfig) map[string]*Principal {
	m := make(map[string]*Principal, len(keys))
	for _, k := range keys {
		m[k.KeyHash] = &Principal{
			ID:       k.Principal,
			IsSystem: true,
		}
	}
	return m
}
