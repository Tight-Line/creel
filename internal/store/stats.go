package store

import (
	"context"
	"fmt"
)

// EntityStats holds row counts for all major entity tables.
type EntityStats struct {
	APIKeyConfigs           int64
	LLMConfigs              int64
	EmbeddingConfigs        int64
	ExtractionPromptConfigs int64
	Topics                  int64
	SystemAccounts          int64
	Documents               int64
	Chunks                  int64
	Memories                int64
}

// StatsStore provides aggregate statistics from the database.
type StatsStore struct {
	pool DBTX
}

// NewStatsStore creates a new StatsStore.
func NewStatsStore(pool DBTX) *StatsStore {
	return &StatsStore{pool: pool}
}

// GetStats returns row counts for all major entity tables in a single query.
func (s *StatsStore) GetStats(ctx context.Context) (*EntityStats, error) {
	var stats EntityStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM api_key_configs),
			(SELECT count(*) FROM llm_configs),
			(SELECT count(*) FROM embedding_configs),
			(SELECT count(*) FROM extraction_prompt_configs),
			(SELECT count(*) FROM topics),
			(SELECT count(*) FROM system_accounts),
			(SELECT count(*) FROM documents),
			(SELECT count(*) FROM chunks),
			(SELECT count(*) FROM memories)
	`).Scan(
		&stats.APIKeyConfigs,
		&stats.LLMConfigs,
		&stats.EmbeddingConfigs,
		&stats.ExtractionPromptConfigs,
		&stats.Topics,
		&stats.SystemAccounts,
		&stats.Documents,
		&stats.Chunks,
		&stats.Memories,
	)
	if err != nil {
		return nil, fmt.Errorf("querying entity stats: %w", err)
	}
	return &stats, nil
}
