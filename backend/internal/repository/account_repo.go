package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

type AccountRepo struct {
	pool *pgxpool.Pool
}

func NewAccountRepo(pool *pgxpool.Pool) *AccountRepo {
	return &AccountRepo{pool: pool}
}

func (r *AccountRepo) Create(ctx context.Context, a *models.Account) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO accounts (id, email, name, company, password_hash, credit_balance, subscription_tier, global_max_per_task, global_max_per_day, is_system_account)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`, a.ID, a.Email, a.Name, a.Company, a.PasswordHash, a.CreditBalance, a.SubscriptionTier, a.GlobalMaxPerTask, a.GlobalMaxPerDay, a.IsSystemAccount).Scan(&a.CreatedAt, &a.UpdatedAt)
}

func (r *AccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Account, error) {
	var a models.Account
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, company, password_hash, credit_balance, subscription_tier, global_max_per_task, global_max_per_day, is_system_account, created_at, updated_at
		FROM accounts WHERE id = $1
	`, id).Scan(&a.ID, &a.Email, &a.Name, &a.Company, &a.PasswordHash, &a.CreditBalance, &a.SubscriptionTier, &a.GlobalMaxPerTask, &a.GlobalMaxPerDay, &a.IsSystemAccount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepo) GetByEmail(ctx context.Context, email string) (*models.Account, error) {
	var a models.Account
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, company, password_hash, credit_balance, subscription_tier, global_max_per_task, global_max_per_day, is_system_account, created_at, updated_at
		FROM accounts WHERE email = $1
	`, email).Scan(&a.ID, &a.Email, &a.Name, &a.Company, &a.PasswordHash, &a.CreditBalance, &a.SubscriptionTier, &a.GlobalMaxPerTask, &a.GlobalMaxPerDay, &a.IsSystemAccount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepo) Update(ctx context.Context, a *models.Account) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE accounts SET email = $2, name = $3, company = $4, password_hash = $5, credit_balance = $6, subscription_tier = $7, global_max_per_task = $8, global_max_per_day = $9, is_system_account = $10, updated_at = now()
		WHERE id = $1
	`, a.ID, a.Email, a.Name, a.Company, a.PasswordHash, a.CreditBalance, a.SubscriptionTier, a.GlobalMaxPerTask, a.GlobalMaxPerDay, a.IsSystemAccount)
	return err
}

func (r *AccountRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM accounts WHERE id = $1", id)
	return err
}

func (r *AccountRepo) List(ctx context.Context) ([]*models.Account, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, name, company, password_hash, credit_balance, subscription_tier, global_max_per_task, global_max_per_day, is_system_account, created_at, updated_at
		FROM accounts ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Account
	for rows.Next() {
		var a models.Account
		if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.Company, &a.PasswordHash, &a.CreditBalance, &a.SubscriptionTier, &a.GlobalMaxPerTask, &a.GlobalMaxPerDay, &a.IsSystemAccount, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &a)
	}
	return list, rows.Err()
}

// GetByIDForUpdate locks the account row for update. Call within a transaction.
func (r *AccountRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Account, error) {
	var a models.Account
	err := tx.QueryRow(ctx, `
		SELECT id, email, name, company, password_hash, credit_balance, subscription_tier, global_max_per_task, global_max_per_day, is_system_account, created_at, updated_at
		FROM accounts WHERE id = $1 FOR UPDATE
	`, id).Scan(&a.ID, &a.Email, &a.Name, &a.Company, &a.PasswordHash, &a.CreditBalance, &a.SubscriptionTier, &a.GlobalMaxPerTask, &a.GlobalMaxPerDay, &a.IsSystemAccount, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateCreditBalance sets account credit_balance. Call after GetByIDForUpdate in same tx.
func (r *AccountRepo) UpdateCreditBalance(ctx context.Context, tx pgx.Tx, id uuid.UUID, creditBalance int) error {
	_, err := tx.Exec(ctx, `
		UPDATE accounts SET credit_balance = $2, updated_at = now() WHERE id = $1
	`, id, creditBalance)
	return err
}

// DeductCredits atomically deducts amount from account if balance >= amount. Returns new balance or error.
func (r *AccountRepo) DeductCredits(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) (newBalance int, err error) {
	err = tx.QueryRow(ctx, `
		UPDATE accounts SET credit_balance = credit_balance - $1, updated_at = now()
		WHERE id = $2 AND credit_balance >= $1
		RETURNING credit_balance
	`, amount, id).Scan(&newBalance)
	return newBalance, err
}

// AddCredits adds amount to account and returns new balance.
func (r *AccountRepo) AddCredits(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) (newBalance int, err error) {
	err = tx.QueryRow(ctx, `
		UPDATE accounts SET credit_balance = credit_balance + $1, updated_at = now()
		WHERE id = $2
		RETURNING credit_balance
	`, amount, id).Scan(&newBalance)
	return newBalance, err
}
