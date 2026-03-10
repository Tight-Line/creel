package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/store"
)

// topicRow returns a mockDBTX whose QueryRow populates a minimal Topic
// (with optional config IDs) on the first call, and returns an error on
// subsequent calls. This simulates TopicStore.Get succeeding for constraint
// validation while allowing Update to fail or succeed as needed.
func topicDBWithExisting(llmID, embID, promptID *string) *mockDBTX {
	callCount := 0
	return &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				// TopicStore.Get scans: id, slug, name, description, owner, created_at, updated_at,
				//   llm_config_id, embedding_config_id, extraction_prompt_config_id
				return &topicRow{llmID: llmID, embID: embID, promptID: promptID}
			}
			return &mockRow{err: nil} // Update succeeds with zero values
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
}

// topicRow implements pgx.Row that scans a Topic with specific config IDs.
type topicRow struct {
	llmID, embID, promptID, vbCfgID *string
}

func (r *topicRow) Scan(dest ...any) error {
	// Matches TopicStore.Get scan order: id, slug, name, description, owner,
	// created_at, updated_at, llm_config_id, embedding_config_id, extraction_prompt_config_id,
	// chunking_strategy, memory_enabled, vector_backend_config_id
	*dest[0].(*string) = "topic-1"   // id
	*dest[1].(*string) = "test-slug" // slug
	*dest[2].(*string) = "Test"      // name
	*dest[3].(*string) = ""          // description
	*dest[4].(*string) = "system:t"  // owner
	// dest[5] and dest[6] are time.Time; zero values are fine
	*dest[7].(**string) = r.llmID
	*dest[8].(**string) = r.embID
	*dest[9].(**string) = r.promptID
	*dest[10].(*[]byte) = nil // chunking_strategy
	*dest[11].(*bool) = false // memory_enabled
	*dest[12].(**string) = r.vbCfgID
	return nil
}

func TestTopicServer_CreateTopic_PromptRequiresLLM(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	promptID := "prompt-1"
	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug:                     "test",
		Name:                     "Test",
		ExtractionPromptConfigId: &promptID,
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestTopicServer_CreateTopic_PromptWithLLM_OK(t *testing.T) {
	// Both LLM and prompt set; should pass validation and hit the store.
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	promptID := "prompt-1"
	llmID := "llm-1"
	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug:                     "test",
		Name:                     "Test",
		LlmConfigId:              &llmID,
		ExtractionPromptConfigId: &promptID,
	})
	// Should get past validation and fail on store (Internal), not InvalidArgument.
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_CreateTopic_WithConfigIDs(t *testing.T) {
	// Exercises the non-nil config ID branches in CreateTopic.
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	llmID := "llm-1"
	embID := "emb-1"
	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug:              "test",
		Name:              "Test",
		LlmConfigId:       &llmID,
		EmbeddingConfigId: &embID,
	})
	// Passes validation, fails on store.
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_UpdateTopic_PromptRequiresLLM(t *testing.T) {
	// Existing topic has no LLM config. Setting a prompt should fail.
	db := topicDBWithExisting(nil, nil, nil)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	promptID := "prompt-1"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                       "topic-1",
		ExtractionPromptConfigId: &promptID,
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestTopicServer_UpdateTopic_PromptWithExistingLLM_OK(t *testing.T) {
	// Existing topic has an LLM config. Setting a prompt should pass validation.
	llmID := "llm-existing"
	db := topicDBWithExisting(&llmID, nil, nil)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	promptID := "prompt-1"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                       "topic-1",
		ExtractionPromptConfigId: &promptID,
	})
	// Should pass validation; the update itself returns a zero-value topic.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopicServer_UpdateTopic_WithLlmConfigId(t *testing.T) {
	// Setting LlmConfigId on update should pass validation and succeed.
	db := topicDBWithExisting(nil, nil, nil)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	llmID := "llm-1"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:          "topic-1",
		LlmConfigId: &llmID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopicServer_UpdateTopic_WithMemoryEnabled(t *testing.T) {
	// Setting MemoryEnabled on update should pass through to the store.
	db := topicDBWithExisting(nil, nil, nil)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	enabled := true
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:            "topic-1",
		MemoryEnabled: &enabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopicServer_UpdateTopic_StoreUpdateError(t *testing.T) {
	// If the store update fails, should return Internal.
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				// TopicStore.Get succeeds.
				return &topicRow{llmID: nil, embID: nil, promptID: nil}
			}
			// TopicStore.Update fails.
			return &mockRow{err: errors.New("db error")}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:   "topic-1",
		Name: "new-name",
	})
	requireCode(t, err, codes.Internal)
}

// embeddingConfigRow implements pgx.Row that scans an EmbeddingConfig.
type embeddingConfigRow struct {
	provider string
	model    string
}

func (r *embeddingConfigRow) Scan(dest ...any) error {
	// EmbeddingConfigStore.Get scans: id, name, provider, model, dimensions,
	//   api_key_config_id, is_default, created_at, updated_at
	*dest[0].(*string) = "emb-id"
	*dest[1].(*string) = "emb-name"
	*dest[2].(*string) = r.provider
	*dest[3].(*string) = r.model
	*dest[4].(*int) = 1536
	*dest[5].(*string) = "ak-id"
	*dest[6].(*bool) = false
	// dest[7] and dest[8] are time.Time; zero values fine
	return nil
}

func TestTopicServer_UpdateTopic_EmbeddingConfigMismatch(t *testing.T) {
	// Existing topic has embedding config "emb-old". Trying to change to "emb-new"
	// with different provider/model should fail.
	oldEmbID := "emb-old"
	db := topicDBWithExisting(nil, &oldEmbID, nil)

	// The embedding store needs to return configs for both old and new IDs.
	embCallCount := 0
	embDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			embCallCount++
			if embCallCount == 1 {
				return &embeddingConfigRow{provider: "openai", model: "text-embedding-3-small"}
			}
			return &embeddingConfigRow{provider: "cohere", model: "embed-english-v3.0"}
		},
	}
	embStore := store.NewEmbeddingConfigStore(embDB)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, embStore)
	ctx := systemCtx()

	newEmbID := "emb-new"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                "topic-1",
		EmbeddingConfigId: &newEmbID,
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestTopicServer_UpdateTopic_EmbeddingConfigSameProviderModel(t *testing.T) {
	// Changing to a new embedding config with same provider+model should succeed.
	oldEmbID := "emb-old"
	db := topicDBWithExisting(nil, &oldEmbID, nil)

	embDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &embeddingConfigRow{provider: "openai", model: "text-embedding-3-small"}
		},
	}
	embStore := store.NewEmbeddingConfigStore(embDB)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, embStore)
	ctx := systemCtx()

	newEmbID := "emb-new"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                "topic-1",
		EmbeddingConfigId: &newEmbID,
	})
	// Should pass validation.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopicServer_UpdateTopic_EmbeddingConfigFetchError(t *testing.T) {
	// If we can't fetch the old embedding config, should return Internal.
	oldEmbID := "emb-old"
	db := topicDBWithExisting(nil, &oldEmbID, nil)

	embDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("db error")}
		},
	}
	embStore := store.NewEmbeddingConfigStore(embDB)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, embStore)
	ctx := systemCtx()

	newEmbID := "emb-new"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                "topic-1",
		EmbeddingConfigId: &newEmbID,
	})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_UpdateTopic_NewEmbeddingConfigFetchError(t *testing.T) {
	// Old config fetches OK, new config fetch fails.
	oldEmbID := "emb-old"
	db := topicDBWithExisting(nil, &oldEmbID, nil)

	embCallCount := 0
	embDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			embCallCount++
			if embCallCount == 1 {
				return &embeddingConfigRow{provider: "openai", model: "text-embedding-3-small"}
			}
			return &mockRow{err: errors.New("db error")}
		},
	}
	embStore := store.NewEmbeddingConfigStore(embDB)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, embStore)
	ctx := systemCtx()

	newEmbID := "emb-new"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                "topic-1",
		EmbeddingConfigId: &newEmbID,
	})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_CreateTopic_WithVectorBackendConfigId(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	vbID := "vb-1"
	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug:                  "test",
		Name:                  "Test",
		VectorBackendConfigId: &vbID,
	})
	// Passes validation, fails on store.
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_UpdateTopic_WithVectorBackendConfigId(t *testing.T) {
	db := topicDBWithExisting(nil, nil, nil)
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{}, nil)
	ctx := systemCtx()

	vbID := "vb-1"
	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:                    "topic-1",
		VectorBackendConfigId: &vbID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopicServer_StoreTopicToProto_AllConfigIDs(t *testing.T) {
	llmID := "llm-1"
	embID := "emb-1"
	promptID := "prompt-1"
	vbID := "vb-1"
	topic := &store.Topic{
		ID:                       "t1",
		Slug:                     "slug",
		Name:                     "name",
		Owner:                    "owner",
		LLMConfigID:              &llmID,
		EmbeddingConfigID:        &embID,
		ExtractionPromptConfigID: &promptID,
		VectorBackendConfigID:    &vbID,
	}
	p := storeTopicToProto(topic)
	if p.GetLlmConfigId() != "llm-1" {
		t.Errorf("LlmConfigId = %q, want llm-1", p.GetLlmConfigId())
	}
	if p.GetEmbeddingConfigId() != "emb-1" {
		t.Errorf("EmbeddingConfigId = %q, want emb-1", p.GetEmbeddingConfigId())
	}
	if p.GetExtractionPromptConfigId() != "prompt-1" {
		t.Errorf("ExtractionPromptConfigId = %q, want prompt-1", p.GetExtractionPromptConfigId())
	}
	if p.GetVectorBackendConfigId() != "vb-1" {
		t.Errorf("VectorBackendConfigId = %q, want vb-1", p.GetVectorBackendConfigId())
	}
}

func TestStoreTopicToProto_ChunkingStrategy(t *testing.T) {
	topic := &store.Topic{
		ID:   "t2",
		Slug: "chunked",
		Name: "Chunked Topic",
		ChunkingStrategy: &store.ChunkingStrategy{
			ChunkSize:    1024,
			ChunkOverlap: 100,
		},
	}
	p := storeTopicToProto(topic)
	if p.GetChunkingStrategy() == nil {
		t.Fatal("expected ChunkingStrategy to be set")
	}
	if p.GetChunkingStrategy().GetChunkSize() != 1024 {
		t.Errorf("ChunkSize = %d, want 1024", p.GetChunkingStrategy().GetChunkSize())
	}
	if p.GetChunkingStrategy().GetChunkOverlap() != 100 {
		t.Errorf("ChunkOverlap = %d, want 100", p.GetChunkingStrategy().GetChunkOverlap())
	}
}

func TestStoreTopicToProto_NilChunkingStrategy(t *testing.T) {
	topic := &store.Topic{
		ID:   "t3",
		Slug: "no-chunking",
		Name: "No Chunking",
	}
	p := storeTopicToProto(topic)
	if p.GetChunkingStrategy() != nil {
		t.Error("expected ChunkingStrategy to be nil")
	}
}
