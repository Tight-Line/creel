package worker

import (
	"strings"
)

// extractJSON strips markdown code fences from LLM responses so the
// contained JSON can be parsed. LLMs frequently wrap JSON output in
// ```json ... ``` blocks despite being told to return raw JSON.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Strip ```json ... ``` or ``` ... ``` fences.
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line.
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	return s
}
