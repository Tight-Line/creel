package slug

import (
	"regexp"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Hello World", "hello-world"},
		{"special chars", "Thrombosis Study 2026!", "thrombosis-study-2026"},
		{"leading trailing", "  --Hello--  ", "hello"},
		{"multiple spaces", "one   two   three", "one-two-three"},
		{"empty", "", ""},
		{"unicode", "caf\u00e9 latt\u00e9", "caf-latt"},
		{"numbers", "test123", "test123"},
		{"all special", "!!!@@@", ""},
		{"mixed case", "FooBar-Baz", "foobar-baz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.in)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	// Pattern: base-XXXX where XXXX is 4 alphanumeric chars.
	pattern := regexp.MustCompile(`^thrombosis-study-2026-[a-z0-9]{4}$`)
	got := Generate("Thrombosis Study 2026")
	if !pattern.MatchString(got) {
		t.Errorf("Generate(\"Thrombosis Study 2026\") = %q, does not match pattern", got)
	}
}

func TestGenerate_EmptyName(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-z0-9]{4}$`)
	got := Generate("")
	if !pattern.MatchString(got) {
		t.Errorf("Generate(\"\") = %q, does not match pattern", got)
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	// Two calls should produce different slugs (probabilistic but safe with 4 chars).
	a := Generate("test")
	b := Generate("test")
	if a == b {
		t.Errorf("Generate produced same slug twice: %q", a)
	}
}
