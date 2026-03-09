package worker

import (
	"testing"
)

func TestSplitText_EmptyText(t *testing.T) {
	chunks := SplitText("", 100, 20)
	if chunks != nil {
		t.Errorf("expected nil for empty text, got %v", chunks)
	}
}

func TestSplitText_ShorterThanChunkSize(t *testing.T) {
	text := "Hello, world!"
	chunks := SplitText(text, 100, 20)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk = %q, want %q", chunks[0], text)
	}
}

func TestSplitText_ExactlyChunkSize(t *testing.T) {
	text := "abcde"
	chunks := SplitText(text, 5, 2)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk = %q, want %q", chunks[0], text)
	}
}

func TestSplitText_WithOverlap(t *testing.T) {
	// 10 chars, chunk_size=5, overlap=2
	// chunk 1: [0:5] = "abcde"
	// chunk 2: [3:8] = "defgh"
	// chunk 3: [6:10] = "ghij"
	text := "abcdefghij"
	chunks := SplitText(text, 5, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "abcde" {
		t.Errorf("chunk[0] = %q, want %q", chunks[0], "abcde")
	}
	if chunks[1] != "defgh" {
		t.Errorf("chunk[1] = %q, want %q", chunks[1], "defgh")
	}
	if chunks[2] != "ghij" {
		t.Errorf("chunk[2] = %q, want %q", chunks[2], "ghij")
	}
}

func TestSplitText_NoOverlap(t *testing.T) {
	text := "abcdefghij"
	chunks := SplitText(text, 5, 0)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "abcde" {
		t.Errorf("chunk[0] = %q, want %q", chunks[0], "abcde")
	}
	if chunks[1] != "fghij" {
		t.Errorf("chunk[1] = %q, want %q", chunks[1], "fghij")
	}
}

func TestSplitText_OverlapExceedsChunkSize(t *testing.T) {
	// When overlap >= chunkSize, it gets clamped to chunkSize/4.
	text := "abcdefghij"
	chunks := SplitText(text, 4, 10)
	// overlap becomes 1 (4/4)
	// chunk 1: [0:4] = "abcd"
	// chunk 2: [3:7] = "defg"
	// chunk 3: [6:10] = "ghij"
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestSplitText_NegativeOverlap(t *testing.T) {
	text := "abcdefghij"
	chunks := SplitText(text, 5, -1)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestSplitText_ZeroChunkSize(t *testing.T) {
	// Zero chunk size should default to DefaultChunkSize.
	text := "short"
	chunks := SplitText(text, 0, 0)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk = %q, want %q", chunks[0], text)
	}
}

func TestSplitText_LargeText(t *testing.T) {
	// Create a 5000-char string with default settings (2048 size, 200 overlap).
	text := ""
	for i := 0; i < 5000; i++ {
		text += "a"
	}
	chunks := SplitText(text, DefaultChunkSize, DefaultChunkOverlap)

	// Verify all text is covered.
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	// First chunk should be exactly DefaultChunkSize.
	if len(chunks[0]) != DefaultChunkSize {
		t.Errorf("first chunk length = %d, want %d", len(chunks[0]), DefaultChunkSize)
	}

	// Verify overlap between adjacent chunks.
	for i := 1; i < len(chunks); i++ {
		prevEnd := chunks[i-1][len(chunks[i-1])-DefaultChunkOverlap:]
		currStart := chunks[i][:DefaultChunkOverlap]
		if prevEnd != currStart {
			t.Errorf("overlap mismatch between chunks %d and %d", i-1, i)
		}
	}
}

func TestChunkingWorker_Type(t *testing.T) {
	w := NewChunkingWorker(nil, nil, nil, nil)
	if w.Type() != "chunking" {
		t.Errorf("Type() = %q, want chunking", w.Type())
	}
}
