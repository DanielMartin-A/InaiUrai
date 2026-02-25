package services

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/inaiurai/backend/internal/models"
)

// ---------------------------------------------------------------------------
// In-memory mocks for EscrowAccountRepo and EscrowCreditRepo.
// These let us test the real EscrowService logic without a database.
// ---------------------------------------------------------------------------

type mockAccount struct {
	mu       sync.Mutex
	accounts map[uuid.UUID]*models.Account
}

func newMockAccount(accs ...*models.Account) *mockAccount {
	m := &mockAccount{accounts: make(map[uuid.UUID]*models.Account)}
	for _, a := range accs {
		cp := *a
		m.accounts[a.ID] = &cp
	}
	return m
}

func (m *mockAccount) GetByIDForUpdate(_ context.Context, _ pgx.Tx, id uuid.UUID) (*models.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %s not found", id)
	}
	cp := *a
	return &cp, nil
}

func (m *mockAccount) DeductCredits(_ context.Context, _ pgx.Tx, id uuid.UUID, amount int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return 0, fmt.Errorf("account %s not found", id)
	}
	if a.CreditBalance < amount {
		return 0, ErrInsufficientFunds
	}
	a.CreditBalance -= amount
	return a.CreditBalance, nil
}

func (m *mockAccount) AddCredits(_ context.Context, _ pgx.Tx, id uuid.UUID, amount int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return 0, fmt.Errorf("account %s not found", id)
	}
	a.CreditBalance += amount
	return a.CreditBalance, nil
}

func (m *mockAccount) balance(id uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accounts[id].CreditBalance
}

// ---

type mockCredit struct {
	mu      sync.Mutex
	entries []*models.CreditLedger
}

func (m *mockCredit) CreateTx(_ context.Context, _ pgx.Tx, c *models.CreditLedger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.entries = append(m.entries, &cp)
	return nil
}

func (m *mockCredit) byType(entryType string) []*models.CreditLedger {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*models.CreditLedger
	for _, e := range m.entries {
		if e.EntryType == entryType {
			out = append(out, e)
		}
	}
	return out
}

func (m *mockCredit) all() []*models.CreditLedger {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*models.CreditLedger, len(m.entries))
	copy(out, m.entries)
	return out
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func acct(id uuid.UUID, balance int) *models.Account {
	return &models.Account{ID: id, CreditBalance: balance}
}

// signedAmount returns the signed delta a ledger entry represents for the
// account that owns it:
//   - escrow_lock deducts  -> negative
//   - everything else adds -> positive
func signedAmount(e *models.CreditLedger) int {
	if e.EntryType == models.CreditEntryEscrowLock {
		return -e.Amount
	}
	return e.Amount
}

// ---------------------------------------------------------------------------
// 1. TestLockCredits
// ---------------------------------------------------------------------------

func TestLockCredits(t *testing.T) {
	requester := uuid.New()
	task := uuid.New()

	accounts := newMockAccount(acct(requester, 1000))
	credits := &mockCredit{}
	svc := NewEscrowService(accounts, credits)

	ctx := context.Background()
	if err := svc.LockCredits(ctx, nil, requester, task, 200); err != nil {
		t.Fatalf("LockCredits: %v", err)
	}

	// Balance should decrease by 200.
	if got := accounts.balance(requester); got != 800 {
		t.Errorf("balance after lock: got %d, want 800", got)
	}

	// Exactly one escrow_lock ledger entry should exist.
	locks := credits.byType(models.CreditEntryEscrowLock)
	if len(locks) != 1 {
		t.Fatalf("escrow_lock entries: got %d, want 1", len(locks))
	}
	if locks[0].Amount != 200 {
		t.Errorf("lock amount: got %d, want 200", locks[0].Amount)
	}
	if locks[0].AccountID != requester {
		t.Error("lock entry should belong to requester account")
	}
	if locks[0].TaskID == nil || *locks[0].TaskID != task {
		t.Error("lock entry should reference the task")
	}

	// Insufficient-funds path.
	if err := svc.LockCredits(ctx, nil, requester, uuid.New(), 9999); err != ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 2. TestSettleTask_Success
// ---------------------------------------------------------------------------

func TestSettleTask_Success(t *testing.T) {
	requester := uuid.New()
	worker := uuid.New()
	platform := models.SystemPlatformAccountID
	task := uuid.New()

	const budget = 100
	const actualCost = 80
	const expectedPlatformFee = 8  // 10% of 80
	const expectedWorkerEarning = 72 // 80 - 8
	const expectedRefund = 20      // 100 - 80

	accounts := newMockAccount(
		acct(requester, 0),
		acct(worker, 0),
		acct(platform, 0),
	)
	credits := &mockCredit{}
	svc := NewEscrowService(accounts, credits)

	ctx := context.Background()
	if err := svc.SettleTask(ctx, nil, task, requester, worker, budget, actualCost); err != nil {
		t.Fatalf("SettleTask: %v", err)
	}

	// Worker gets 90%.
	if got := accounts.balance(worker); got != expectedWorkerEarning {
		t.Errorf("worker balance: got %d, want %d", got, expectedWorkerEarning)
	}
	earnings := credits.byType(models.CreditEntryTaskEarning)
	if len(earnings) != 1 || earnings[0].Amount != expectedWorkerEarning {
		t.Errorf("task_earning entry: got amount %d, want %d", safeAmount(earnings), expectedWorkerEarning)
	}

	// Platform gets 10% (CRITICAL).
	if got := accounts.balance(platform); got != expectedPlatformFee {
		t.Errorf("platform balance: got %d, want %d", got, expectedPlatformFee)
	}
	fees := credits.byType(models.CreditEntryPlatformFee)
	if len(fees) != 1 || fees[0].Amount != expectedPlatformFee {
		t.Errorf("platform_fee entry: got amount %d, want %d", safeAmount(fees), expectedPlatformFee)
	}
	if fees[0].AccountID != platform {
		t.Errorf("platform_fee entry should go to SystemPlatformAccountID (%s), got %s", platform, fees[0].AccountID)
	}

	// Requester gets refund of remainder.
	if got := accounts.balance(requester); got != expectedRefund {
		t.Errorf("requester balance: got %d, want %d", got, expectedRefund)
	}
	releases := credits.byType(models.CreditEntryEscrowRelease)
	if len(releases) != 1 || releases[0].Amount != expectedRefund {
		t.Errorf("escrow_release entry: got amount %d, want %d", safeAmount(releases), expectedRefund)
	}
}

// ---------------------------------------------------------------------------
// 3. TestSettleError
// ---------------------------------------------------------------------------

func TestSettleError(t *testing.T) {
	requester := uuid.New()
	worker := uuid.New()
	task := uuid.New()

	const budget = 100

	// Requester starts at 0 (credits already escrowed).
	accounts := newMockAccount(
		acct(requester, 0),
		acct(worker, 500),
	)
	credits := &mockCredit{}
	svc := NewEscrowService(accounts, credits)

	ctx := context.Background()
	if err := svc.SettleError(ctx, nil, task, requester, budget); err != nil {
		t.Fatalf("SettleError: %v", err)
	}

	// Requester gets full refund.
	if got := accounts.balance(requester); got != budget {
		t.Errorf("requester balance after refund: got %d, want %d", got, budget)
	}
	refunds := credits.byType(models.CreditEntryRefund)
	if len(refunds) != 1 || refunds[0].Amount != budget {
		t.Errorf("refund entry: got amount %d, want %d", safeAmount(refunds), budget)
	}

	// Worker balance unchanged (0 to worker).
	if got := accounts.balance(worker); got != 500 {
		t.Errorf("worker balance should be unchanged: got %d, want 500", got)
	}

	// No task_earning or platform_fee entries.
	if n := len(credits.byType(models.CreditEntryTaskEarning)); n != 0 {
		t.Errorf("expected 0 task_earning entries, got %d", n)
	}
	if n := len(credits.byType(models.CreditEntryPlatformFee)); n != 0 {
		t.Errorf("expected 0 platform_fee entries, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 4. TestLedgerIntegrity
//    Full cycle: lock → settle success → assert that
//    SUM(signed ledger entries per account) + initial == current balance.
// ---------------------------------------------------------------------------

func TestLedgerIntegrity(t *testing.T) {
	requester := uuid.New()
	worker := uuid.New()
	platform := models.SystemPlatformAccountID
	task := uuid.New()

	const initialRequester = 1000
	const initialWorker = 200
	const initialPlatform = 0
	const budget = 100
	const actualCost = 80

	accounts := newMockAccount(
		acct(requester, initialRequester),
		acct(worker, initialWorker),
		acct(platform, initialPlatform),
	)
	credits := &mockCredit{}
	svc := NewEscrowService(accounts, credits)

	ctx := context.Background()

	// Step 1: Lock.
	if err := svc.LockCredits(ctx, nil, requester, task, budget); err != nil {
		t.Fatalf("LockCredits: %v", err)
	}

	// Step 2: Settle success.
	if err := svc.SettleTask(ctx, nil, task, requester, worker, budget, actualCost); err != nil {
		t.Fatalf("SettleTask: %v", err)
	}

	// Build per-account ledger sums.
	deltas := map[uuid.UUID]int{}
	for _, e := range credits.all() {
		deltas[e.AccountID] += signedAmount(e)
	}

	initials := map[uuid.UUID]int{
		requester: initialRequester,
		worker:    initialWorker,
		platform:  initialPlatform,
	}

	for id, initial := range initials {
		expected := initial + deltas[id]
		got := accounts.balance(id)
		if got != expected {
			t.Errorf("account %s: initial(%d) + ledger_sum(%d) = %d, but balance is %d",
				id, initial, deltas[id], expected, got)
		}
	}

	// Global conservation: total credits in system must equal initial total.
	totalInitial := initialRequester + initialWorker + initialPlatform
	totalNow := accounts.balance(requester) + accounts.balance(worker) + accounts.balance(platform)
	if totalNow != totalInitial {
		t.Errorf("credit conservation violated: initial total %d, current total %d", totalInitial, totalNow)
	}
}

// ---------------------------------------------------------------------------
// safeAmount returns the first entry's Amount or -1 if slice is empty.
// ---------------------------------------------------------------------------

func safeAmount(entries []*models.CreditLedger) int {
	if len(entries) == 0 {
		return -1
	}
	return entries[0].Amount
}
