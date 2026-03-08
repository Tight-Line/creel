package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExtractionPromptConfigStore handles extraction prompt configuration persistence.
type ExtractionPromptConfigStore struct {
	pool DBTX
}

// NewExtractionPromptConfigStore creates a new extraction prompt config store.
func NewExtractionPromptConfigStore(pool DBTX) *ExtractionPromptConfigStore {
	return &ExtractionPromptConfigStore{pool: pool}
}

// ExtractionPromptConfig represents a stored extraction prompt configuration.
type ExtractionPromptConfig struct {
	ID          string
	Name        string
	Prompt      string
	Description string
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Create inserts a new extraction prompt config.
func (s *ExtractionPromptConfigStore) Create(ctx context.Context, name, prompt, description string, isDefault bool) (*ExtractionPromptConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isDefault {
		if _, err := tx.Exec(ctx, `UPDATE extraction_prompt_configs SET is_default = false WHERE is_default = true`); err != nil {
			return nil, fmt.Errorf("clearing previous default: %w", err)
		}
	}

	var c ExtractionPromptConfig
	err = tx.QueryRow(ctx,
		`INSERT INTO extraction_prompt_configs (name, prompt, description, is_default)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, prompt, description, is_default, created_at, updated_at`,
		name, prompt, description, isDefault,
	).Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting extraction prompt config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// Get retrieves an extraction prompt config by ID.
func (s *ExtractionPromptConfigStore) Get(ctx context.Context, id string) (*ExtractionPromptConfig, error) {
	var c ExtractionPromptConfig
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, prompt, description, is_default, created_at, updated_at
		 FROM extraction_prompt_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("extraction prompt config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying extraction prompt config: %w", err)
	}
	return &c, nil
}

// List returns all extraction prompt configs.
func (s *ExtractionPromptConfigStore) List(ctx context.Context) ([]ExtractionPromptConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, prompt, description, is_default, created_at, updated_at
		 FROM extraction_prompt_configs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing extraction prompt configs: %w", err)
	}
	defer rows.Close()

	var configs []ExtractionPromptConfig
	for rows.Next() {
		var c ExtractionPromptConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning extraction prompt config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Update modifies an extraction prompt config.
func (s *ExtractionPromptConfigStore) Update(ctx context.Context, id, name, prompt, description string) (*ExtractionPromptConfig, error) {
	var c ExtractionPromptConfig
	err := s.pool.QueryRow(ctx,
		`UPDATE extraction_prompt_configs
		 SET name = COALESCE(NULLIF($2, ''), name),
		     prompt = COALESCE(NULLIF($3, ''), prompt),
		     description = COALESCE(NULLIF($4, ''), description),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, prompt, description, is_default, created_at, updated_at`,
		id, name, prompt, description,
	).Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("extraction prompt config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating extraction prompt config: %w", err)
	}
	return &c, nil
}

// Delete removes an extraction prompt config by ID.
func (s *ExtractionPromptConfigStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM extraction_prompt_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting extraction prompt config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("extraction prompt config not found")
	}
	return nil
}

// SetDefault marks the given config as the default, clearing any previous default.
func (s *ExtractionPromptConfigStore) SetDefault(ctx context.Context, id string) (*ExtractionPromptConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE extraction_prompt_configs SET is_default = false WHERE is_default = true`); err != nil {
		return nil, fmt.Errorf("clearing previous default: %w", err)
	}

	var c ExtractionPromptConfig
	err = tx.QueryRow(ctx,
		`UPDATE extraction_prompt_configs SET is_default = true, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, prompt, description, is_default, created_at, updated_at`,
		id,
	).Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("extraction prompt config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("setting default extraction prompt config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// GetDefault returns the default extraction prompt config, or nil if none is set.
func (s *ExtractionPromptConfigStore) GetDefault(ctx context.Context) (*ExtractionPromptConfig, error) {
	var c ExtractionPromptConfig
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, prompt, description, is_default, created_at, updated_at
		 FROM extraction_prompt_configs WHERE is_default = true`,
	).Scan(&c.ID, &c.Name, &c.Prompt, &c.Description, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying default extraction prompt config: %w", err)
	}
	return &c, nil
}
