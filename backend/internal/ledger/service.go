package ledger

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service interface {
	PlaceEscrowHold(ctx context.Context, tx pgx.Tx, jobID, requesterID uuid.UUID, amountCents int64) error
	ReleaseEscrow(ctx context.Context, jobID, providerID uuid.UUID) error
	RefundEscrow(ctx context.Context, jobID uuid.UUID) error
}

type service struct {
	repo *Repository
}

func NewService(repo *Repository) Service {
	return &service{repo: repo}
}

var _ Service = (*service)(nil)

func (s *service) PlaceEscrowHold(ctx context.Context, tx pgx.Tx, jobID, requesterID uuid.UUID, amountCents int64) error {
	return s.repo.PlaceEscrowHold(ctx, tx, jobID, requesterID, amountCents)
}

func (s *service) ReleaseEscrow(ctx context.Context, jobID, providerID uuid.UUID) error {
	return s.repo.ReleaseEscrow(ctx, jobID, providerID)
}

func (s *service) RefundEscrow(ctx context.Context, jobID uuid.UUID) error {
	return s.repo.RefundEscrow(ctx, jobID)
}

// ErrInsufficientFunds is returned when the requester's balance is too low for the escrow hold.
var ErrInsufficientFunds = errInsufficientFunds
