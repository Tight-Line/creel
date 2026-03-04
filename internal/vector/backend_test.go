package vector

import "testing"

// Compile-time interface check: ensure Backend is a valid interface type.
// This will fail to compile if Backend has any issues.
func TestBackendInterfaceCompiles(t *testing.T) {
	var _ Backend = (Backend)(nil)
}
