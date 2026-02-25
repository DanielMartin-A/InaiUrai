package auth

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new account and returns the created Account.
func (r *Repository) Create(ctx context.Context, email, passwordHash, displayName, role string) (*Account, error) {
	var id uuid.UUID
	var creditBalance int
	row := r.pool.QueryRow(ctx, `
		INSERT INTO accounts (email, password_hash, name, company, is_system_account, subscription_tier)
		VALUES ($1, $2, $3, '', FALSE, 'free')
		RETURNING id, credit_balance
	`, email, passwordHash, displayName)
	if err := row.Scan(&id, &creditBalance); err != nil {
		return nil, err
	}
	return &Account{
		ID:           id,
		Email:        email,
		DisplayName:  displayName,
		Role:         role,
		BalanceCents: int64(creditBalance),
		HoldCents:    0,
	}, nil
}

// GetByEmail returns the account and password hash for login. Returns nil if not found.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*Account, string, error) {
	var a Account
	var passwordHash string
	var creditBalance int
	var name *string
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, name, credit_balance, password_hash
		FROM accounts WHERE email = $1
	`, email)
	if err := row.Scan(&a.ID, &a.Email, &name, &creditBalance, &passwordHash); err != nil {
		if err.Error() == "no rows in result set" {
			return nil, "", nil
		}
		return nil, "", err
	}
	if name != nil {
		a.DisplayName = *name
	}
	a.Role = "requester"
	a.BalanceCents = int64(creditBalance)
	a.HoldCents = 0
	return &a, passwordHash, nil
}
