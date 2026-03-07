package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Tight-Line/creel/internal/auth"
)

// SystemAccountStore handles system account and key persistence.
type SystemAccountStore struct {
	pool DBTX
}

// NewSystemAccountStore creates a new store backed by the given pool.
func NewSystemAccountStore(pool DBTX) *SystemAccountStore {
	return &SystemAccountStore{pool: pool}
}

// SystemAccount represents a stored system account.
type SystemAccount struct {
	ID          string
	Name        string
	Description string
	Principal   string
	CreatedAt   time.Time
}

// SystemAccountKey represents a stored API key.
type SystemAccountKey struct {
	ID             string
	AccountID      string
	KeyHash        string
	KeyPrefix      string
	Status         string // "active", "grace_period", "revoked"
	GraceExpiresAt *time.Time
	CreatedAt      time.Time
	RevokedAt      *time.Time
}

// Create inserts a new system account and its initial key.
// Returns the account and the raw API key (shown once).
func (s *SystemAccountStore) Create(ctx context.Context, name, description string) (*SystemAccount, string, error) {
	principal := "system:" + name

	rawKey, keyHash, keyPrefix, _ := auth.GenerateAPIKey()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var acct SystemAccount
	err = tx.QueryRow(ctx,
		`INSERT INTO system_accounts (name, description, principal)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, description, principal, created_at`,
		name, description, principal,
	).Scan(&acct.ID, &acct.Name, &acct.Description, &acct.Principal, &acct.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("inserting system account: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO system_account_keys (account_id, key_hash, key_prefix, status)
		 VALUES ($1, $2, $3, 'active')`,
		acct.ID, keyHash, keyPrefix,
	)
	if err != nil {
		return nil, "", fmt.Errorf("inserting API key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("committing transaction: %w", err)
	}

	return &acct, rawKey, nil
}

// List returns all system accounts.
func (s *SystemAccountStore) List(ctx context.Context) ([]SystemAccount, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, principal, created_at
		 FROM system_accounts ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing system accounts: %w", err)
	}
	defer rows.Close()

	var accounts []SystemAccount
	for rows.Next() {
		var a SystemAccount
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Principal, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning system account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// Delete removes a system account and cascades to its keys.
func (s *SystemAccountStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM system_accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting system account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("system account not found")
	}
	return nil
}

// RotateKey generates a new key for the account. If gracePeriod > 0, old active
// keys enter grace period; otherwise they are revoked immediately.
func (s *SystemAccountStore) RotateKey(ctx context.Context, accountID string, gracePeriod time.Duration) (string, error) {
	rawKey, keyHash, keyPrefix, _ := auth.GenerateAPIKey()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if gracePeriod > 0 {
		graceExpiry := time.Now().Add(gracePeriod)
		_, err = tx.Exec(ctx,
			`UPDATE system_account_keys
			 SET status = 'grace_period', grace_expires_at = $1
			 WHERE account_id = $2 AND status = 'active'`,
			graceExpiry, accountID,
		)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE system_account_keys
			 SET status = 'revoked', revoked_at = now()
			 WHERE account_id = $1 AND status = 'active'`,
			accountID,
		)
	}
	if err != nil {
		return "", fmt.Errorf("updating old keys: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO system_account_keys (account_id, key_hash, key_prefix, status)
		 VALUES ($1, $2, $3, 'active')`,
		accountID, keyHash, keyPrefix,
	)
	if err != nil {
		return "", fmt.Errorf("inserting new key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("committing transaction: %w", err)
	}

	return rawKey, nil
}

// RevokeKey immediately revokes all active keys for the account.
func (s *SystemAccountStore) RevokeKey(ctx context.Context, accountID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE system_account_keys
		 SET status = 'revoked', revoked_at = now()
		 WHERE account_id = $1 AND status IN ('active', 'grace_period')`,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("revoking keys: %w", err)
	}
	return nil
}

// LookupKeyHash implements auth.KeyLookup. It resolves a key hash to a principal
// by checking active and grace-period keys.
func (s *SystemAccountStore) LookupKeyHash(ctx context.Context, hash string) (*auth.Principal, error) {
	var principal string
	var keyStatus string
	var graceExpiresAt *time.Time

	err := s.pool.QueryRow(ctx,
		`SELECT sa.principal, sak.status, sak.grace_expires_at
		 FROM system_account_keys sak
		 JOIN system_accounts sa ON sa.id = sak.account_id
		 WHERE sak.key_hash = $1 AND sak.status IN ('active', 'grace_period')`,
		hash,
	).Scan(&principal, &keyStatus, &graceExpiresAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("looking up key: %w", err)
	}

	// Check if grace period has expired.
	if keyStatus == "grace_period" && graceExpiresAt != nil && time.Now().After(*graceExpiresAt) {
		return nil, nil
	}

	return &auth.Principal{
		ID:       principal,
		IsSystem: true,
	}, nil
}
