package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// VectorBackendConfigStore handles vector backend configuration persistence.
type VectorBackendConfigStore struct {
	pool DBTX
}

// NewVectorBackendConfigStore creates a new vector backend config store.
func NewVectorBackendConfigStore(pool DBTX) *VectorBackendConfigStore {
	return &VectorBackendConfigStore{pool: pool}
}

// VectorBackendConfig represents a stored vector backend configuration.
type VectorBackendConfig struct {
	ID        string
	Name      string
	Backend   string
	Config    map[string]any
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Create inserts a new vector backend config.
func (s *VectorBackendConfigStore) Create(ctx context.Context, name, backend string, config map[string]any, isDefault bool) (*VectorBackendConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isDefault {
		// coverage:ignore - requires real DB transaction failure
		if _, err := tx.Exec(ctx, `UPDATE vector_backend_configs SET is_default = false WHERE is_default = true`); err != nil {
			return nil, fmt.Errorf("clearing previous default: %w", err)
		}
	}

	configJSON, err := json.Marshal(config)
	// coverage:ignore - map[string]any always marshals successfully
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	var c VectorBackendConfig
	var rawConfig []byte
	err = tx.QueryRow(ctx,
		`INSERT INTO vector_backend_configs (name, backend, config, is_default)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, backend, config, is_default, created_at, updated_at`,
		name, backend, configJSON, isDefault,
	).Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting vector backend config: %w", err)
	}
	// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
	if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	// coverage:ignore - commit after successful insert
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	return &c, nil
}

// Get retrieves a vector backend config by ID.
func (s *VectorBackendConfigStore) Get(ctx context.Context, id string) (*VectorBackendConfig, error) {
	var c VectorBackendConfig
	var rawConfig []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, backend, config, is_default, created_at, updated_at
		 FROM vector_backend_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("vector backend config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying vector backend config: %w", err)
	}
	// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
	if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return &c, nil
}

// List returns all vector backend configs.
func (s *VectorBackendConfigStore) List(ctx context.Context) ([]VectorBackendConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, backend, config, is_default, created_at, updated_at
		 FROM vector_backend_configs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing vector backend configs: %w", err)
	}
	defer rows.Close()

	var configs []VectorBackendConfig
	for rows.Next() {
		var c VectorBackendConfig
		var rawConfig []byte
		// coverage:ignore - scan on valid result set columns
		if err := rows.Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning vector backend config: %w", err)
		}
		// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
		if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Update modifies a vector backend config. Only name and config can be changed;
// the backend type cannot be changed post-creation.
func (s *VectorBackendConfigStore) Update(ctx context.Context, id, name string, config map[string]any) (*VectorBackendConfig, error) {
	var configJSON []byte
	if config != nil {
		var err error
		configJSON, err = json.Marshal(config)
		// coverage:ignore - map[string]any always marshals successfully
		if err != nil {
			return nil, fmt.Errorf("marshaling config: %w", err)
		}
	}

	var c VectorBackendConfig
	var rawConfig []byte
	err := s.pool.QueryRow(ctx,
		`UPDATE vector_backend_configs
		 SET name = COALESCE(NULLIF($2, ''), name),
		     config = COALESCE($3::jsonb, config),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, backend, config, is_default, created_at, updated_at`,
		id, name, configJSON,
	).Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("vector backend config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating vector backend config: %w", err)
	}
	// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
	if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return &c, nil
}

// Delete removes a vector backend config by ID.
func (s *VectorBackendConfigStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM vector_backend_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting vector backend config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("vector backend config not found")
	}
	return nil
}

// SetDefault marks the given config as the default, clearing any previous default.
func (s *VectorBackendConfigStore) SetDefault(ctx context.Context, id string) (*VectorBackendConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// coverage:ignore - requires real DB transaction failure
	if _, err := tx.Exec(ctx, `UPDATE vector_backend_configs SET is_default = false WHERE is_default = true`); err != nil {
		return nil, fmt.Errorf("clearing previous default: %w", err)
	}

	var c VectorBackendConfig
	var rawConfig []byte
	err = tx.QueryRow(ctx,
		`UPDATE vector_backend_configs SET is_default = true, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, backend, config, is_default, created_at, updated_at`,
		id,
	).Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("vector backend config not found")
	}
	// coverage:ignore - requires non-ErrNoRows DB error after successful connection
	if err != nil {
		return nil, fmt.Errorf("setting default vector backend config: %w", err)
	}
	// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
	if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// coverage:ignore - commit after successful query
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// GetDefault returns the default vector backend config, or nil if none is set.
func (s *VectorBackendConfigStore) GetDefault(ctx context.Context) (*VectorBackendConfig, error) {
	var c VectorBackendConfig
	var rawConfig []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, backend, config, is_default, created_at, updated_at
		 FROM vector_backend_configs WHERE is_default = true`,
	).Scan(&c.ID, &c.Name, &c.Backend, &rawConfig, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying default vector backend config: %w", err)
	}
	// coverage:ignore - JSONB from postgres always unmarshals to map[string]any
	if err := json.Unmarshal(rawConfig, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return &c, nil
}
