package services

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/inaiurai/backend/internal/models"
)

// ErrInsufficientFunds is returned when the account balance is too low for the requested lock.
var ErrInsufficientFunds = errors.New("insufficient funds")

// EscrowService performs double-entry credit escrow using the accounts and credit_ledger tables.
type EscrowService struct {
	AccountRepo EscrowAccountRepo
	CreditRepo  EscrowCreditRepo
}

// EscrowAccountRepo is the minimal account repository interface for escrow.
type EscrowAccountRepo interface {
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Account, error)
	DeductCredits(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) (newBalance int, err error)
	AddCredits(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) (newBalance int, err error)
}

// EscrowCreditRepo is the minimal credit ledger interface for escrow.
type EscrowCreditRepo interface {
	CreateTx(ctx context.Context, tx pgx.Tx, c *models.CreditLedger) error
}

// NewEscrowService returns a new EscrowService.
func NewEscrowService(accountRepo EscrowAccountRepo, creditRepo EscrowCreditRepo) *EscrowService {
	return &EscrowService{AccountRepo: accountRepo, CreditRepo: creditRepo}
}

// LockCredits locks the account row (SELECT FOR UPDATE), deducts amount, and inserts an escrow_lock ledger entry.
// Call within a transaction.
func (s *EscrowService) LockCredits(ctx context.Context, tx pgx.Tx, accountID, taskID uuid.UUID, amount int) error {
	acc, err := s.AccountRepo.GetByIDForUpdate(ctx, tx, accountID)
	if err != nil {
		return err
	}
	if acc.CreditBalance < amount {
		return ErrInsufficientFunds
	}
	newBalance, err := s.AccountRepo.DeductCredits(ctx, tx, accountID, amount)
	if err != nil {
		return err
	}
	entry := &models.CreditLedger{
		ID:           uuid.New(),
		AccountID:     accountID,
		TaskID:        &taskID,
		EntryType:     models.CreditEntryEscrowLock,
		Amount:        amount,
		BalanceAfter:  intPtr(newBalance),
	}
	return s.CreditRepo.CreateTx(ctx, tx, entry)
}

// SettleTask applies success/partial settlement: credit worker (task_earning), platform (platform_fee), and refund remainder to requester (escrow_release).
// requesterID and workerID are account UUIDs. budget is the task budget; actualCost is the amount charged.
// Locks requester, worker, and platform accounts in deterministic order to avoid deadlock.
func (s *EscrowService) SettleTask(ctx context.Context, tx pgx.Tx, taskID, requesterID, workerID uuid.UUID, budget, actualCost int) error {
	platformFee := actualCost * 10 / 100
	workerEarning := actualCost - platformFee
	remainder := budget - actualCost
	if remainder < 0 {
		remainder = 0
	}

	// Lock all affected accounts in deterministic order (by UUID)
	ids := []uuid.UUID{requesterID, workerID, models.SystemPlatformAccountID}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if _, err := s.AccountRepo.GetByIDForUpdate(ctx, tx, id); err != nil {
			return err
		}
	}

	// Credit worker (task_earning)
	newWorker, err := s.AccountRepo.AddCredits(ctx, tx, workerID, workerEarning)
	if err != nil {
		return err
	}
	if err := s.CreditRepo.CreateTx(ctx, tx, &models.CreditLedger{
		ID: uuid.New(), AccountID: workerID, TaskID: &taskID,
		EntryType: models.CreditEntryTaskEarning, Amount: workerEarning, BalanceAfter: intPtr(newWorker),
	}); err != nil {
		return err
	}

	// Credit platform (platform_fee)
	newPlatform, err := s.AccountRepo.AddCredits(ctx, tx, models.SystemPlatformAccountID, platformFee)
	if err != nil {
		return err
	}
	if err := s.CreditRepo.CreateTx(ctx, tx, &models.CreditLedger{
		ID: uuid.New(), AccountID: models.SystemPlatformAccountID, TaskID: &taskID,
		EntryType: models.CreditEntryPlatformFee, Amount: platformFee, BalanceAfter: intPtr(newPlatform),
	}); err != nil {
		return err
	}

	// Refund requester remainder (escrow_release)
	if remainder > 0 {
		newReq, err := s.AccountRepo.AddCredits(ctx, tx, requesterID, remainder)
		if err != nil {
			return err
		}
		if err := s.CreditRepo.CreateTx(ctx, tx, &models.CreditLedger{
			ID: uuid.New(), AccountID: requesterID, TaskID: &taskID,
			EntryType: models.CreditEntryEscrowRelease, Amount: remainder, BalanceAfter: intPtr(newReq),
		}); err != nil {
			return err
		}
	}
	return nil
}

// SettleError fully refunds the task budget to the requester (refund entry).
func (s *EscrowService) SettleError(ctx context.Context, tx pgx.Tx, taskID, requesterID uuid.UUID, budget int) error {
	if budget <= 0 {
		return nil
	}
	if _, err := s.AccountRepo.GetByIDForUpdate(ctx, tx, requesterID); err != nil {
		return err
	}
	newBalance, err := s.AccountRepo.AddCredits(ctx, tx, requesterID, budget)
	if err != nil {
		return err
	}
	return s.CreditRepo.CreateTx(ctx, tx, &models.CreditLedger{
		ID: uuid.New(), AccountID: requesterID, TaskID: &taskID,
		EntryType: models.CreditEntryRefund, Amount: budget, BalanceAfter: intPtr(newBalance),
	})
}

// RefundFailed is the same as SettleError: full refund to requester.
func (s *EscrowService) RefundFailed(ctx context.Context, tx pgx.Tx, taskID, requesterID uuid.UUID, budget int) error {
	return s.SettleError(ctx, tx, taskID, requesterID, budget)
}

func intPtr(n int) *int { return &n }
