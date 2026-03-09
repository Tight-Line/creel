package main

import (
	"fmt"
	"strings"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// FormatCitedResult formats a search result with its citation information.
// Returns a string like:
//
//	[Source: "Paper Title" by Author, http://example.com]
//	[role]: content text
//
// If no citation is available, falls back to the plain "[role]: content" format.
func FormatCitedResult(r *pb.SearchResult) string {
	chunk := r.GetChunk()
	if chunk == nil {
		return ""
	}

	role := extractRole(r)
	content := chunk.GetContent()

	citation := r.GetDocumentCitation()
	if citation == nil || !hasCitationInfo(citation) {
		return fmt.Sprintf("[%s]: %s", role, content)
	}

	citationLine := formatCitationLine(citation)
	return fmt.Sprintf("%s\n[%s]: %s", citationLine, role, content)
}

// hasCitationInfo returns true if the citation has any displayable fields.
func hasCitationInfo(c *pb.DocumentCitation) bool {
	return c.GetName() != "" || c.GetAuthor() != "" || c.GetUrl() != ""
}

// formatCitationLine builds the "[Source: ...]" line from a citation.
func formatCitationLine(c *pb.DocumentCitation) string {
	var parts []string
	if name := c.GetName(); name != "" {
		parts = append(parts, fmt.Sprintf("%q", name))
	}
	if author := c.GetAuthor(); author != "" {
		parts = append(parts, fmt.Sprintf("by %s", author))
	}
	if url := c.GetUrl(); url != "" {
		parts = append(parts, url)
	}
	return fmt.Sprintf("[Source: %s]", strings.Join(parts, ", "))
}
