package main

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

func TestFormatCitedResult_NoCitation(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user"})
	r := &pb.SearchResult{
		Chunk: &pb.Chunk{Content: "Hello world", Metadata: meta},
	}
	got := FormatCitedResult(r)
	if got != "[user]: Hello world" {
		t.Errorf("expected '[user]: Hello world', got %q", got)
	}
}

func TestFormatCitedResult_FullCitation(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "assistant"})
	r := &pb.SearchResult{
		Chunk: &pb.Chunk{Content: "The answer is 42", Metadata: meta},
		DocumentCitation: &pb.DocumentCitation{
			Name:   "Deep Thought",
			Author: "Douglas Adams",
			Url:    "https://example.com/hitchhiker",
		},
	}
	got := FormatCitedResult(r)
	expected := "[Source: \"Deep Thought\", by Douglas Adams, https://example.com/hitchhiker]\n[assistant]: The answer is 42"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestFormatCitedResult_NameOnly(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user"})
	r := &pb.SearchResult{
		Chunk: &pb.Chunk{Content: "content", Metadata: meta},
		DocumentCitation: &pb.DocumentCitation{
			Name: "My Doc",
		},
	}
	got := FormatCitedResult(r)
	expected := "[Source: \"My Doc\"]\n[user]: content"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestFormatCitedResult_URLOnly(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user"})
	r := &pb.SearchResult{
		Chunk: &pb.Chunk{Content: "content", Metadata: meta},
		DocumentCitation: &pb.DocumentCitation{
			Url: "https://example.com",
		},
	}
	got := FormatCitedResult(r)
	expected := "[Source: https://example.com]\n[user]: content"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestFormatCitedResult_EmptyCitation(t *testing.T) {
	meta, _ := structpb.NewStruct(map[string]any{"role": "user"})
	r := &pb.SearchResult{
		Chunk:            &pb.Chunk{Content: "hello", Metadata: meta},
		DocumentCitation: &pb.DocumentCitation{},
	}
	got := FormatCitedResult(r)
	if got != "[user]: hello" {
		t.Errorf("expected '[user]: hello', got %q", got)
	}
}

func TestFormatCitedResult_NilChunk(t *testing.T) {
	r := &pb.SearchResult{}
	got := FormatCitedResult(r)
	if got != "" {
		t.Errorf("expected empty string for nil chunk, got %q", got)
	}
}

func TestHasCitationInfo(t *testing.T) {
	tests := []struct {
		name   string
		c      *pb.DocumentCitation
		expect bool
	}{
		{"empty", &pb.DocumentCitation{}, false},
		{"name", &pb.DocumentCitation{Name: "x"}, true},
		{"author", &pb.DocumentCitation{Author: "x"}, true},
		{"url", &pb.DocumentCitation{Url: "x"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasCitationInfo(tc.c)
			if got != tc.expect {
				t.Errorf("expected %v, got %v", tc.expect, got)
			}
		})
	}
}

func TestFormatCitationLine_AllFields(t *testing.T) {
	c := &pb.DocumentCitation{
		Name:   "Title",
		Author: "Author",
		Url:    "https://example.com",
	}
	got := formatCitationLine(c)
	expected := `[Source: "Title", by Author, https://example.com]`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatCitationLine_AuthorOnly(t *testing.T) {
	c := &pb.DocumentCitation{Author: "Jane"}
	got := formatCitationLine(c)
	if got != "[Source: by Jane]" {
		t.Errorf("expected '[Source: by Jane]', got %q", got)
	}
}
