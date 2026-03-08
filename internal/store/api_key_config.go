package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Tight-Line/creel/internal/crypto"
)

// APIKeyConfigStore handles API key configuration persistence.
type APIKeyConfigStore struct {
	pool      DBTX
	encryptor *crypto.Encryptor
}

// NewAPIKeyConfigStore creates a new API key config store.
func NewAPIKeyConfigStore(pool DBTX, encryptor *crypto.Encryptor) *APIKeyConfigStore {
	return &APIKeyConfigStore{pool: pool, encryptor: encryptor}
}

// APIKeyConfig represents a stored API key configuration.
type APIKeyConfig struct {
	ID        string
	Name      string
	Provider  string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Create inserts a new API key config with the key encrypted at rest.
func (s *APIKeyConfigStore) Create(ctx context.Context, name, provider string, apiKey []byte, isDefault bool) (*APIKeyConfig, error) {
	encryptedKey, nonce, err := s.encryptor.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypting API key: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isDefault {
		if _, err := tx.Exec(ctx, `UPDATE api_key_configs SET is_default = false WHERE is_default = true`); err != nil {
			return nil, fmt.Errorf("clearing previous default: %w", err)
		}
	}

	var c APIKeyConfig
	err = tx.QueryRow(ctx,
		`INSERT INTO api_key_configs (name, provider, encrypted_key, key_nonce, is_default)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, name, provider, is_default, created_at, updated_at`,
		name, provider, encryptedKey, nonce, isDefault,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting API key config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// Get retrieves an API key config by ID (without the decrypted key).
func (s *APIKeyConfigStore) Get(ctx context.Context, id string) (*APIKeyConfig, error) {
	var c APIKeyConfig
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider, is_default, created_at, updated_at
		 FROM api_key_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("API key config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying API key config: %w", err)
	}
	return &c, nil
}

// List returns all API key configs (without decrypted keys).
func (s *APIKeyConfigStore) List(ctx context.Context) ([]APIKeyConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, provider, is_default, created_at, updated_at
		 FROM api_key_configs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing API key configs: %w", err)
	}
	defer rows.Close()

	var configs []APIKeyConfig
	for rows.Next() {
		var c APIKeyConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning API key config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Update modifies an API key config. If apiKey is non-nil, the key is re-encrypted.
func (s *APIKeyConfigStore) Update(ctx context.Context, id, name, provider string, apiKey []byte) (*APIKeyConfig, error) {
	if apiKey != nil {
		encryptedKey, nonce, err := s.encryptor.Encrypt(apiKey)
		if err != nil {
			return nil, fmt.Errorf("encrypting API key: %w", err)
		}

		var c APIKeyConfig
		err = s.pool.QueryRow(ctx,
			`UPDATE api_key_configs
			 SET name = COALESCE(NULLIF($2, ''), name),
			     provider = COALESCE(NULLIF($3, ''), provider),
			     encrypted_key = $4, key_nonce = $5, updated_at = now()
			 WHERE id = $1
			 RETURNING id, name, provider, is_default, created_at, updated_at`,
			id, name, provider, encryptedKey, nonce,
		).Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("API key config not found")
		}
		if err != nil {
			return nil, fmt.Errorf("updating API key config: %w", err)
		}
		return &c, nil
	}

	var c APIKeyConfig
	err := s.pool.QueryRow(ctx,
		`UPDATE api_key_configs
		 SET name = COALESCE(NULLIF($2, ''), name),
		     provider = COALESCE(NULLIF($3, ''), provider),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, provider, is_default, created_at, updated_at`,
		id, name, provider,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("API key config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating API key config: %w", err)
	}
	return &c, nil
}

// Delete removes an API key config by ID.
func (s *APIKeyConfigStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM api_key_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting API key config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("API key config not found")
	}
	return nil
}

// SetDefault marks the given config as the default, clearing any previous default.
func (s *APIKeyConfigStore) SetDefault(ctx context.Context, id string) (*APIKeyConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE api_key_configs SET is_default = false WHERE is_default = true`); err != nil {
		return nil, fmt.Errorf("clearing previous default: %w", err)
	}

	var c APIKeyConfig
	err = tx.QueryRow(ctx,
		`UPDATE api_key_configs SET is_default = true, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, provider, is_default, created_at, updated_at`,
		id,
	).Scan(&c.ID, &c.Name, &c.Provider, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("API key config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("setting default API key config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &c, nil
}

// GetDecrypted retrieves the decrypted API key for server-side use.
func (s *APIKeyConfigStore) GetDecrypted(ctx context.Context, id string) ([]byte, error) {
	var encryptedKey, nonce []byte
	err := s.pool.QueryRow(ctx,
		`SELECT encrypted_key, key_nonce FROM api_key_configs WHERE id = $1`, id,
	).Scan(&encryptedKey, &nonce)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("API key config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying encrypted key: %w", err)
	}

	plaintext, err := s.encryptor.Decrypt(encryptedKey, nonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}
	return plaintext, nil
}
