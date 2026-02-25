package ledger

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var escrowAccountID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

var errInsufficientFunds = errors.New("insufficient funds")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// PlaceEscrowHold runs inside the caller's transaction. It:
// a) Verifies requester balance_cents >= amountCents (via atomic UPDATE with condition)
// b) Deducts balance_cents and adds to hold_cents on the requester account
// c) Inserts an ESCROW_HOLD record into transactions
// d) Inserts a record into escrow_holds
func (r *Repository) PlaceEscrowHold(ctx context.Context, tx pgx.Tx, jobID, requesterID uuid.UUID, amountCents int64) error {
	result, err := tx.Exec(ctx, `
		UPDATE accounts
		SET balance_cents = balance_cents - $1, hold_cents = hold_cents + $1
		WHERE id = $2 AND balance_cents >= $1
	`, amountCents, requesterID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errInsufficientFunds
	}
	var holdTxID uuid.UUID
	row := tx.QueryRow(ctx, `
		INSERT INTO transactions (tx_type, job_id, debit_account_id, credit_account_id, amount_cents)
		VALUES ('ESCROW_HOLD', $1, $2, $3, $4)
		RETURNING id
	`, jobID, requesterID, escrowAccountID, amountCents)
	if err := row.Scan(&holdTxID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO escrow_holds (job_id, requester_id, amount_cents, status, hold_tx_id)
		VALUES ($1, $2, $3, 'HELD', $4)
	`, jobID, requesterID, amountCents, holdTxID)
	return err
}

// ReleaseEscrow runs in its own transaction: pays provider from escrow and marks hold released.
func (r *Repository) ReleaseEscrow(ctx context.Context, jobID, providerID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var requesterID uuid.UUID
	var amountCents int64
	var holdTxID uuid.UUID
	row := tx.QueryRow(ctx, `
		SELECT requester_id, amount_cents, hold_tx_id FROM escrow_holds WHERE job_id = $1 AND status = 'HELD'
	`, jobID)
	if err := row.Scan(&requesterID, &amountCents, &holdTxID); err != nil {
		return err
	}
	var releaseTxID uuid.UUID
	row = tx.QueryRow(ctx, `
		INSERT INTO transactions (tx_type, job_id, debit_account_id, credit_account_id, amount_cents)
		VALUES ('ESCROW_RELEASE', $1, $2, $3, $4)
		RETURNING id
	`, jobID, escrowAccountID, providerID, amountCents)
	if err := row.Scan(&releaseTxID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE escrow_holds SET status = 'RELEASED', release_tx_id = $1 WHERE job_id = $2
	`, releaseTxID, jobID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE accounts SET hold_cents = hold_cents - $1 WHERE id = $2`, amountCents, requesterID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, amountCents, providerID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RefundEscrow returns funds from escrow to the requester (e.g. job failed).
func (r *Repository) RefundEscrow(ctx context.Context, jobID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var requesterID uuid.UUID
	var amountCents int64
	row := tx.QueryRow(ctx, `
		SELECT requester_id, amount_cents FROM escrow_holds WHERE job_id = $1 AND status = 'HELD'
	`, jobID)
	if err := row.Scan(&requesterID, &amountCents); err != nil {
		return err
	}
	row = tx.QueryRow(ctx, `
		INSERT INTO transactions (tx_type, job_id, debit_account_id, credit_account_id, amount_cents)
		VALUES ('ESCROW_REFUND', $1, $2, $3, $4)
		RETURNING id
	`, jobID, escrowAccountID, requesterID, amountCents)
	var refundTxID uuid.UUID
	if err := row.Scan(&refundTxID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE escrow_holds SET status = 'REFUNDED', release_tx_id = $1 WHERE job_id = $2
	`, refundTxID, jobID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE accounts SET hold_cents = hold_cents - $1, balance_cents = balance_cents + $1 WHERE id = $2`, amountCents, requesterID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
