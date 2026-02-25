package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

type CreditRepo struct {
	pool *pgxpool.Pool
}

func NewCreditRepo(pool *pgxpool.Pool) *CreditRepo {
	return &CreditRepo{pool: pool}
}

func (r *CreditRepo) Create(ctx context.Context, c *models.CreditLedger) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO credit_ledger (id, account_id, task_id, entry_type, amount, balance_after)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`, c.ID, c.AccountID, c.TaskID, c.EntryType, c.Amount, c.BalanceAfter).Scan(&c.CreatedAt)
}

// CreateTx inserts a ledger entry inside the given transaction.
func (r *CreditRepo) CreateTx(ctx context.Context, tx pgx.Tx, c *models.CreditLedger) error {
	return tx.QueryRow(ctx, `
		INSERT INTO credit_ledger (id, account_id, task_id, entry_type, amount, balance_after)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`, c.ID, c.AccountID, c.TaskID, c.EntryType, c.Amount, c.BalanceAfter).Scan(&c.CreatedAt)
}

func (r *CreditRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.CreditLedger, error) {
	var c models.CreditLedger
	err := r.pool.QueryRow(ctx, `
		SELECT id, account_id, task_id, entry_type, amount, balance_after, created_at
		FROM credit_ledger WHERE id = $1
	`, id).Scan(&c.ID, &c.AccountID, &c.TaskID, &c.EntryType, &c.Amount, &c.BalanceAfter, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CreditRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM credit_ledger WHERE id = $1", id)
	return err
}

func (r *CreditRepo) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.CreditLedger, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, account_id, task_id, entry_type, amount, balance_after, created_at
		FROM credit_ledger WHERE account_id = $1 ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.CreditLedger
	for rows.Next() {
		var c models.CreditLedger
		if err := rows.Scan(&c.ID, &c.AccountID, &c.TaskID, &c.EntryType, &c.Amount, &c.BalanceAfter, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}

func (r *CreditRepo) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.CreditLedger, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, account_id, task_id, entry_type, amount, balance_after, created_at
		FROM credit_ledger WHERE task_id = $1 ORDER BY created_at DESC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.CreditLedger
	for rows.Next() {
		var c models.CreditLedger
		if err := rows.Scan(&c.ID, &c.AccountID, &c.TaskID, &c.EntryType, &c.Amount, &c.BalanceAfter, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}
