package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// EmbeddingConfigStore handles embedding configuration persistence.
type EmbeddingConfigStore struct {
	pool DBTX
}

// NewEmbeddingConfigStore creates a new embedding config store.
func NewEmbeddingConfigStore(pool DBTX) *EmbeddingConfigStore {
	return &EmbeddingConfigStore{pool: pool}
}

// EmbeddingConfig represents a stored embedding configuration.
type EmbeddingConfig struct {
	ID             string
	Name           string
	Provider       string
	Model          string
	Dimensions     int
	APIKeyConfigID string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Create inserts a new embedding config.
func (s *EmbeddingConfigStore) Create(ctx context.Context, name, provider, model string, dimensions int, apiKeyConfigID string, isDefault bool) (*EmbeddingConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isDefault {
		if _, err := tx.Exec(ctx, `UPDATE embedding_configs SET is_default = false WHERE is_default = true`); err != nil {
			return nil, fmt.Errorf("clearing previous default: %w", err)
		}
	}

	var c EmbeddingConfig
	err = tx.QueryRow(ctx,
		`INSERT INTO embedding_configs (name, provider, model, dimensions, api_key_config_id, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at`,
		name, provider, model, dimensions, apiKeyConfigID, isDefault,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting embedding config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// Get retrieves an embedding config by ID.
func (s *EmbeddingConfigStore) Get(ctx context.Context, id string) (*EmbeddingConfig, error) {
	var c EmbeddingConfig
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at
		 FROM embedding_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("embedding config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying embedding config: %w", err)
	}
	return &c, nil
}

// List returns all embedding configs.
func (s *EmbeddingConfigStore) List(ctx context.Context) ([]EmbeddingConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at
		 FROM embedding_configs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing embedding configs: %w", err)
	}
	defer rows.Close()

	var configs []EmbeddingConfig
	for rows.Next() {
		var c EmbeddingConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning embedding config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Update modifies an embedding config. Only name and api_key_config_id can be changed.
func (s *EmbeddingConfigStore) Update(ctx context.Context, id, name, apiKeyConfigID string) (*EmbeddingConfig, error) {
	var c EmbeddingConfig
	err := s.pool.QueryRow(ctx,
		`UPDATE embedding_configs
		 SET name = COALESCE(NULLIF($2, ''), name),
		     api_key_config_id = COALESCE(NULLIF($3, '')::uuid, api_key_config_id),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at`,
		id, name, apiKeyConfigID,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("embedding config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating embedding config: %w", err)
	}
	return &c, nil
}

// Delete removes an embedding config by ID.
func (s *EmbeddingConfigStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM embedding_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting embedding config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("embedding config not found")
	}
	return nil
}

// SetDefault marks the given config as the default, clearing any previous default.
func (s *EmbeddingConfigStore) SetDefault(ctx context.Context, id string) (*EmbeddingConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE embedding_configs SET is_default = false WHERE is_default = true`); err != nil {
		return nil, fmt.Errorf("clearing previous default: %w", err)
	}

	var c EmbeddingConfig
	err = tx.QueryRow(ctx,
		`UPDATE embedding_configs SET is_default = true, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at`,
		id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("embedding config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("setting default embedding config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// GetDefault returns the default embedding config, or nil if none is set.
func (s *EmbeddingConfigStore) GetDefault(ctx context.Context) (*EmbeddingConfig, error) {
	var c EmbeddingConfig
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider, model, dimensions, api_key_config_id, is_default, created_at, updated_at
		 FROM embedding_configs WHERE is_default = true`,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &c.Dimensions, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying default embedding config: %w", err)
	}
	return &c, nil
}
