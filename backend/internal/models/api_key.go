package models

import (
	"github.com/google/uuid"
)

type APIKey struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`
	KeyHash   string    `json:"-"`
	KeyPrefix string    `json:"key_prefix"`
	IsActive  bool      `json:"is_active"`
}
