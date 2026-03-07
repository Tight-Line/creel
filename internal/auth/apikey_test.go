package auth

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, prefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !strings.HasPrefix(raw, "creel_ak_") {
		t.Errorf("raw key missing prefix: %q", raw)
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if !strings.HasPrefix(raw, prefix) {
		t.Errorf("prefix %q not a prefix of raw key", prefix)
	}
	// Hash should be deterministic.
	if HashAPIKey(raw) != hash {
		t.Error("HashAPIKey(raw) != returned hash")
	}
}

func TestIsAPIKey(t *testing.T) {
	if !IsAPIKey("creel_ak_abc123") {
		t.Error("expected true for valid prefix")
	}
	if IsAPIKey("creel_ak_") {
		t.Error("expected false for prefix-only")
	}
	if IsAPIKey("Bearer xyz") {
		t.Error("expected false for Bearer token")
	}
}

func TestAPIKeyValidator(t *testing.T) {
	raw, hash, _, _ := GenerateAPIKey()
	staticKeys := map[string]*Principal{
		hash: {ID: "system:test", IsSystem: true},
	}

	v := NewAPIKeyValidator(staticKeys, nil)

	// Valid key.
	p, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p == nil || p.ID != "system:test" {
		t.Errorf("principal = %v, want system:test", p)
	}

	// Invalid key.
	p, err = v.Validate(context.Background(), "creel_ak_invalid")
	if err != nil {
		t.Fatalf("Validate invalid: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil principal for invalid key, got %v", p)
	}
}

func TestStaticKeysFromConfig_Empty(t *testing.T) {
	v := NewAPIKeyValidator(nil, nil)
	p, err := v.Validate(context.Background(), "creel_ak_anything")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p != nil {
		t.Error("expected nil principal")
	}
}

// mockKeyLookup implements KeyLookup for testing.
type mockKeyLookup struct {
	principal *Principal
	err       error
}

func (m *mockKeyLookup) LookupKeyHash(_ context.Context, _ string) (*Principal, error) {
	return m.principal, m.err
}

func TestAPIKeyValidator_DynamicLookup(t *testing.T) {
	lookup := &mockKeyLookup{
		principal: &Principal{ID: "system:dynamic", IsSystem: true},
	}
	v := NewAPIKeyValidator(nil, lookup)

	p, err := v.Validate(context.Background(), "creel_ak_dynamic_test_key_1234567890ab")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p == nil || p.ID != "system:dynamic" {
		t.Errorf("principal = %v, want system:dynamic", p)
	}
}

func TestAPIKeyValidator_DynamicLookupError(t *testing.T) {
	lookup := &mockKeyLookup{
		err: context.DeadlineExceeded,
	}
	v := NewAPIKeyValidator(nil, lookup)

	_, err := v.Validate(context.Background(), "creel_ak_dynamic_test_key_1234567890ab")
	if err == nil {
		t.Fatal("expected error from dynamic lookup")
	}
}
