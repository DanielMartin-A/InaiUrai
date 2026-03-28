package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

type OrgContext struct {
	ID          uuid.UUID       `json:"id"`
	OrgID       uuid.UUID       `json:"org_id"`
	ContextType string          `json:"context_type"`
	Content     json.RawMessage `json:"content"`
	Source      string          `json:"source,omitempty"`
}
