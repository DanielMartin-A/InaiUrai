package models

import (
	"time"

	"github.com/google/uuid"
)

// Credit ledger entry_type enums from .cursorrules.
const (
	CreditEntryEscrowLock    = "escrow_lock"
	CreditEntryEscrowRelease = "escrow_release"
	CreditEntryTaskEarning   = "task_earning"
	CreditEntryPlatformFee   = "platform_fee"
	CreditEntryRefund        = "refund"
)

type CreditLedger struct {
	ID           uuid.UUID  `json:"id"`
	AccountID    uuid.UUID  `json:"account_id"`
	TaskID       *uuid.UUID `json:"task_id,omitempty"`
	EntryType    string     `json:"entry_type"`
	Amount       int        `json:"amount"`
	BalanceAfter *int       `json:"balance_after,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
