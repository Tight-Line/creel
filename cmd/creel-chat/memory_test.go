package main

import (
	"strings"
	"testing"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

func TestFormatMemoryPrompt_Empty(t *testing.T) {
	got := FormatMemoryPrompt(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatMemoryPrompt_EmptySlice(t *testing.T) {
	got := FormatMemoryPrompt([]*pb.Memory{})
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatMemoryPrompt_SingleMemory(t *testing.T) {
	memories := []*pb.Memory{
		{Content: "User likes fly fishing"},
	}
	got := FormatMemoryPrompt(memories)
	if !strings.Contains(got, "WHAT I KNOW ABOUT YOU") {
		t.Error("expected 'WHAT I KNOW ABOUT YOU' header")
	}
	if !strings.Contains(got, "- User likes fly fishing") {
		t.Error("expected memory content with bullet")
	}
}

func TestFormatMemoryPrompt_MultipleMemories(t *testing.T) {
	memories := []*pb.Memory{
		{Content: "Lives in Maine"},
		{Content: "Likes skiing"},
		{Content: "Works in software"},
	}
	got := FormatMemoryPrompt(memories)
	if !strings.Contains(got, "- Lives in Maine") {
		t.Error("missing first memory")
	}
	if !strings.Contains(got, "- Likes skiing") {
		t.Error("missing second memory")
	}
	if !strings.Contains(got, "- Works in software") {
		t.Error("missing third memory")
	}
}

func TestFormatMemoryPrompt_SkipsEmptyContent(t *testing.T) {
	memories := []*pb.Memory{
		{Content: ""},
		{Content: "Real fact"},
	}
	got := FormatMemoryPrompt(memories)
	if !strings.Contains(got, "- Real fact") {
		t.Error("expected non-empty memory")
	}
	// Count bullet lines: should be exactly one.
	bulletCount := strings.Count(got, "\n- ")
	if bulletCount != 1 {
		t.Errorf("expected 1 bullet line, got %d", bulletCount)
	}
}
