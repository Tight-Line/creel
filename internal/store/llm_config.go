package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// LLMConfigStore handles LLM configuration persistence.
type LLMConfigStore struct {
	pool DBTX
}

// NewLLMConfigStore creates a new LLM config store.
func NewLLMConfigStore(pool DBTX) *LLMConfigStore {
	return &LLMConfigStore{pool: pool}
}

// LLMConfig represents a stored LLM configuration.
type LLMConfig struct {
	ID             string
	Name           string
	Provider       string
	Model          string
	Parameters     map[string]string
	APIKeyConfigID string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Create inserts a new LLM config.
func (s *LLMConfigStore) Create(ctx context.Context, name, provider, model string, params map[string]string, apiKeyConfigID string, isDefault bool) (*LLMConfig, error) {
	paramsJSON, err := json.Marshal(params)
	// coverage:ignore - map[string]string cannot fail to marshal
	if err != nil {
		return nil, fmt.Errorf("marshaling parameters: %w", err)
	}
	if params == nil {
		paramsJSON = []byte("{}")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isDefault {
		if _, err := tx.Exec(ctx, `UPDATE llm_configs SET is_default = false WHERE is_default = true`); err != nil {
			return nil, fmt.Errorf("clearing previous default: %w", err)
		}
	}

	var c LLMConfig
	var rawParams []byte
	err = tx.QueryRow(ctx,
		`INSERT INTO llm_configs (name, provider, model, parameters, api_key_config_id, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at`,
		name, provider, model, paramsJSON, apiKeyConfigID, isDefault,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting LLM config: %w", err)
	}

	// coverage:ignore - DB JSONB is valid JSON
	if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
		return nil, fmt.Errorf("unmarshaling parameters: %w", err)
	}

	// coverage:ignore - transaction commit on healthy connection
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// Get retrieves an LLM config by ID.
func (s *LLMConfigStore) Get(ctx context.Context, id string) (*LLMConfig, error) {
	var c LLMConfig
	var rawParams []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at
		 FROM llm_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("LLM config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying LLM config: %w", err)
	}

	// coverage:ignore - DB JSONB is valid JSON
	if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
		return nil, fmt.Errorf("unmarshaling parameters: %w", err)
	}

	return &c, nil
}

// List returns all LLM configs.
func (s *LLMConfigStore) List(ctx context.Context) ([]LLMConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at
		 FROM llm_configs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing LLM configs: %w", err)
	}
	defer rows.Close()

	var configs []LLMConfig
	for rows.Next() {
		var c LLMConfig
		var rawParams []byte
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning LLM config: %w", err)
		}
		// coverage:ignore - DB JSONB is valid JSON
		if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
			return nil, fmt.Errorf("unmarshaling parameters: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Update modifies an LLM config.
func (s *LLMConfigStore) Update(ctx context.Context, id, name, provider, model string, params map[string]string, apiKeyConfigID string) (*LLMConfig, error) {
	var paramsJSON []byte
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		// coverage:ignore - map[string]string cannot fail to marshal
		if err != nil {
			return nil, fmt.Errorf("marshaling parameters: %w", err)
		}
	}

	var c LLMConfig
	var rawParams []byte
	var err error

	if paramsJSON != nil {
		err = s.pool.QueryRow(ctx,
			`UPDATE llm_configs
			 SET name = COALESCE(NULLIF($2, ''), name),
			     provider = COALESCE(NULLIF($3, ''), provider),
			     model = COALESCE(NULLIF($4, ''), model),
			     parameters = $5,
			     api_key_config_id = COALESCE(NULLIF($6, '')::uuid, api_key_config_id),
			     updated_at = now()
			 WHERE id = $1
			 RETURNING id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at`,
			id, name, provider, model, paramsJSON, apiKeyConfigID,
		).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	} else {
		err = s.pool.QueryRow(ctx,
			`UPDATE llm_configs
			 SET name = COALESCE(NULLIF($2, ''), name),
			     provider = COALESCE(NULLIF($3, ''), provider),
			     model = COALESCE(NULLIF($4, ''), model),
			     api_key_config_id = COALESCE(NULLIF($5, '')::uuid, api_key_config_id),
			     updated_at = now()
			 WHERE id = $1
			 RETURNING id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at`,
			id, name, provider, model, apiKeyConfigID,
		).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	}

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("LLM config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating LLM config: %w", err)
	}

	// coverage:ignore - DB JSONB is valid JSON
	if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
		return nil, fmt.Errorf("unmarshaling parameters: %w", err)
	}

	return &c, nil
}

// Delete removes an LLM config by ID.
func (s *LLMConfigStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM llm_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting LLM config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("LLM config not found")
	}
	return nil
}

// SetDefault marks the given config as the default, clearing any previous default.
func (s *LLMConfigStore) SetDefault(ctx context.Context, id string) (*LLMConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE llm_configs SET is_default = false WHERE is_default = true`); err != nil {
		return nil, fmt.Errorf("clearing previous default: %w", err)
	}

	var c LLMConfig
	var rawParams []byte
	err = tx.QueryRow(ctx,
		`UPDATE llm_configs SET is_default = true, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at`,
		id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("LLM config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("setting default LLM config: %w", err)
	}

	// coverage:ignore - DB JSONB is valid JSON
	if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
		return nil, fmt.Errorf("unmarshaling parameters: %w", err)
	}

	// coverage:ignore - transaction commit on healthy connection
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// GetDefault returns the default LLM config, or nil if none is set.
func (s *LLMConfigStore) GetDefault(ctx context.Context) (*LLMConfig, error) {
	var c LLMConfig
	var rawParams []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider, model, parameters, api_key_config_id, is_default, created_at, updated_at
		 FROM llm_configs WHERE is_default = true`,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.Model, &rawParams, &c.APIKeyConfigID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying default LLM config: %w", err)
	}

	// coverage:ignore - DB JSONB is valid JSON
	if err := json.Unmarshal(rawParams, &c.Parameters); err != nil {
		return nil, fmt.Errorf("unmarshaling parameters: %w", err)
	}

	return &c, nil
}
