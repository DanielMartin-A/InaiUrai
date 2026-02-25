package models

import (
	"time"

	"github.com/google/uuid"
)

// Constants from .cursorrules CONSTANTS section.
var (
	SystemPlatformAccountID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	AdminAccountID          = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// SeedAPIKey is the bootstrap API key value (hash this before comparing to api_keys.key_hash).
const SeedAPIKey = "inai_seed_bootstrap_key_do_not_share"

type Account struct {
	ID                  uuid.UUID `json:"id"`
	Email               string    `json:"email"`
	Name                string    `json:"name"`
	Company             string    `json:"company"`
	PasswordHash         string    `json:"-"`
	CreditBalance       int       `json:"credit_balance"`
	SubscriptionTier    string    `json:"subscription_tier"`
	GlobalMaxPerTask    *int      `json:"global_max_per_task,omitempty"`
	GlobalMaxPerDay     *int      `json:"global_max_per_day,omitempty"`
	IsSystemAccount     bool      `json:"is_system_account"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
