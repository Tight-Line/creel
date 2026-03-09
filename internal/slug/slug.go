// Package slug provides URL-friendly slug generation from names.
package slug

import (
	"crypto/rand"
	"regexp"
	"strings"
)

var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)
	leadingTrailing = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a name into a URL-friendly slug.
// It lowercases the input, replaces non-alphanumeric characters with hyphens,
// and trims leading/trailing hyphens.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = leadingTrailing.ReplaceAllString(s, "")
	return s
}

// Generate creates a slug from the given name with a random 4-character suffix.
// If name is empty, returns just the random suffix.
func Generate(name string) string {
	base := Slugify(name)
	suffix := randomSuffix(4)
	if base == "" {
		return suffix
	}
	return base + "-" + suffix
}

// randomSuffix generates a random alphanumeric lowercase string of length n.
func randomSuffix(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b) // coverage:ignore - crypto/rand.Read only errors on system entropy failure
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
