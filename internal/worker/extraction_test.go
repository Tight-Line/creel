package worker

import (
	"strings"
	"testing"
)

func TestExtractText_PlainText(t *testing.T) {
	data := []byte("Hello, world!")
	got, err := ExtractText(data, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello, world!" {
		t.Errorf("got %q, want %q", got, "Hello, world!")
	}
}

func TestExtractText_PlainTextWithCharset(t *testing.T) {
	data := []byte("Hello!")
	got, err := ExtractText(data, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello!" {
		t.Errorf("got %q, want %q", got, "Hello!")
	}
}

func TestExtractText_HTML(t *testing.T) {
	data := []byte(`<html><head><title>Test</title><style>body{}</style></head><body><h1>Hello</h1><p>World</p><script>alert(1)</script></body></html>`)
	got, err := ExtractText(data, "text/html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected to contain 'Hello', got %q", got)
	}
	if !strings.Contains(got, "World") {
		t.Errorf("expected to contain 'World', got %q", got)
	}
	// Script and style content should be stripped.
	if strings.Contains(got, "alert") {
		t.Errorf("script content should be stripped, got %q", got)
	}
	if strings.Contains(got, "body{}") {
		t.Errorf("style content should be stripped, got %q", got)
	}
}

func TestExtractText_XHTML(t *testing.T) {
	data := []byte(`<html><body><p>XHTML content</p></body></html>`)
	got, err := ExtractText(data, "application/xhtml+xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "XHTML content") {
		t.Errorf("got %q, want to contain 'XHTML content'", got)
	}
}

func TestExtractText_Unsupported(t *testing.T) {
	_, err := ExtractText([]byte("data"), "application/pdf")
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention 'unsupported': %v", err)
	}
}

func TestExtractText_HTMLTitle(t *testing.T) {
	data := []byte(`<html><head><title>My Title</title></head><body>Body text</body></html>`)
	got, err := ExtractText(data, "text/html; charset=utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "My Title") {
		t.Errorf("expected to contain 'My Title', got %q", got)
	}
	if !strings.Contains(got, "Body text") {
		t.Errorf("expected to contain 'Body text', got %q", got)
	}
}
