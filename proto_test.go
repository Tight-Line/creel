package creel_test

import (
	"testing"

	creelv1 "github.com/Tight-Line/creel/gen/creel/v1"
)

// TestGeneratedTypesCompile proves codegen output compiles for all 7 services.
func TestGeneratedTypesCompile(t *testing.T) {
	// admin
	_ = &creelv1.HealthRequest{}
	_ = &creelv1.CreateSystemAccountRequest{}

	// chunk
	_ = &creelv1.IngestChunksRequest{}
	_ = &creelv1.Chunk{}

	// compaction
	_ = &creelv1.CompactRequest{}
	_ = &creelv1.UncompactRequest{}

	// document
	_ = &creelv1.CreateDocumentRequest{}
	_ = &creelv1.Document{}

	// link
	_ = &creelv1.CreateLinkRequest{}
	_ = &creelv1.Link{}

	// retrieval
	_ = &creelv1.SearchRequest{}
	_ = &creelv1.GetContextRequest{}

	// topic
	_ = &creelv1.CreateTopicRequest{}
	_ = &creelv1.Topic{}
	_ = &creelv1.TopicGrant{}
}
