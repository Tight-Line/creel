package main

import (
	"strings"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// FormatMemoryPrompt formats a list of memories into a section for the system prompt.
// Returns an empty string if there are no memories.
func FormatMemoryPrompt(memories []*pb.Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n--- WHAT I KNOW ABOUT YOU ---\n")
	for _, m := range memories {
		content := m.GetContent()
		if content == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	sb.WriteString("--- END WHAT I KNOW ABOUT YOU ---")
	return sb.String()
}
