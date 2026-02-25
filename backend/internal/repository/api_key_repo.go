package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

type APIKeyRepo struct {
	pool *pgxpool.Pool
}

func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

// APIKeyWithAccount is returned by FindByKeyHash (api_key joined with account).
type APIKeyWithAccount struct {
	APIKey  models.APIKey
	Account models.Account
}

func (r *APIKeyRepo) Create(ctx context.Context, k *models.APIKey) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO api_keys (id, account_id, key_hash, key_prefix, is_active)
		VALUES ($1, $2, $3, $4, $5)
	`, k.ID, k.AccountID, k.KeyHash, k.KeyPrefix, k.IsActive)
	return err
}

func (r *APIKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var k models.APIKey
	err := r.pool.QueryRow(ctx, `
		SELECT id, account_id, key_hash, key_prefix, is_active
		FROM api_keys WHERE id = $1
	`, id).Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.KeyPrefix, &k.IsActive)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *APIKeyRepo) Update(ctx context.Context, k *models.APIKey) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE api_keys SET account_id = $2, key_hash = $3, key_prefix = $4, is_active = $5
		WHERE id = $1
	`, k.ID, k.AccountID, k.KeyHash, k.KeyPrefix, k.IsActive)
	return err
}

func (r *APIKeyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM api_keys WHERE id = $1", id)
	return err
}

// ListByAccountID returns all API keys for the given account.
func (r *APIKeyRepo) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.APIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, account_id, key_hash, key_prefix, is_active
		FROM api_keys WHERE account_id = $1 ORDER BY key_prefix
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.KeyPrefix, &k.IsActive); err != nil {
			return nil, err
		}
		list = append(list, &k)
	}
	if list == nil {
		list = []*models.APIKey{}
	}
	return list, rows.Err()
}

// FindByKeyHash returns the api_key and joined account for the given key hash, or nil if not found or inactive.
func (r *APIKeyRepo) FindByKeyHash(ctx context.Context, keyHash string) (*APIKeyWithAccount, error) {
	var out APIKeyWithAccount
	err := r.pool.QueryRow(ctx, `
		SELECT k.id, k.account_id, k.key_hash, k.key_prefix, k.is_active,
		       ac.id, ac.email, ac.name, ac.company, ac.password_hash, ac.credit_balance, ac.subscription_tier, ac.global_max_per_task, ac.global_max_per_day, ac.is_system_account, ac.created_at, ac.updated_at
		FROM api_keys k
		INNER JOIN accounts ac ON ac.id = k.account_id
		WHERE k.key_hash = $1 AND k.is_active = TRUE
	`, keyHash).Scan(
		&out.APIKey.ID, &out.APIKey.AccountID, &out.APIKey.KeyHash, &out.APIKey.KeyPrefix, &out.APIKey.IsActive,
		&out.Account.ID, &out.Account.Email, &out.Account.Name, &out.Account.Company, &out.Account.PasswordHash, &out.Account.CreditBalance, &out.Account.SubscriptionTier, &out.Account.GlobalMaxPerTask, &out.Account.GlobalMaxPerDay, &out.Account.IsSystemAccount, &out.Account.CreatedAt, &out.Account.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
