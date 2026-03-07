package auth

import (
	"testing"

	"github.com/Tight-Line/creel/internal/config"
)

func TestStaticKeysFromConfig(t *testing.T) {
	keys := []config.APIKeyConfig{
		{Name: "agent-a", KeyHash: "hash-a", Principal: "system:agent-a"},
		{Name: "agent-b", KeyHash: "hash-b", Principal: "system:agent-b"},
	}

	m := StaticKeysFromConfig(keys)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["hash-a"].ID != "system:agent-a" {
		t.Errorf("hash-a principal = %q, want system:agent-a", m["hash-a"].ID)
	}
	if !m["hash-b"].IsSystem {
		t.Error("expected IsSystem = true for hash-b")
	}
}

func TestStaticKeysFromConfig_Nil(t *testing.T) {
	m := StaticKeysFromConfig(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}
