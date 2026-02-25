package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Task status and output_status enums from .cursorrules.
const (
	TaskStatusCreated      = "created"
	TaskStatusMatching     = "matching"
	TaskStatusDispatched   = "dispatched"
	TaskStatusInProgress   = "in_progress"
	TaskStatusCompleted    = "completed"
	TaskStatusFailed       = "failed"

	TaskOutputStatusSuccess = "success"
	TaskOutputStatusPartial = "partial"
	TaskOutputStatusError   = "error"
)

// Routing preference for worker matching (used by matching service).
const (
	RoutingFastest  = "fastest"
	RoutingCheapest = "cheapest"
	RoutingAuto     = "auto"
)

type Task struct {
	ID                 uuid.UUID       `json:"id"`
	RequesterAgentID   uuid.UUID       `json:"requester_agent_id"`
	WorkerAgentID      *uuid.UUID      `json:"worker_agent_id,omitempty"`
	CapabilityRequired string          `json:"capability_required"`
	InputPayload       json.RawMessage `json:"input_payload"`
	OutputPayload      json.RawMessage `json:"output_payload,omitempty"`
	OutputStatus       string          `json:"output_status"`
	Status             string          `json:"status"`
	Budget             int             `json:"budget"`
	ActualCost         *int            `json:"actual_cost,omitempty"`
	PlatformFee        *int            `json:"platform_fee,omitempty"`
	Deadline           *time.Time      `json:"deadline,omitempty"`
	RetryCount         int             `json:"retry_count"`
	RoutingPreference  string          `json:"routing_preference,omitempty"` // fastest | cheapest | auto (default)
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}
