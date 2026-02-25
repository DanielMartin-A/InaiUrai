package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Agent role and availability enums from .cursorrules.
const (
	AgentRoleRequester = "requester"
	AgentRoleWorker    = "worker"
	AgentRoleBoth      = "both"

	AgentAvailabilityOnline  = "online"
	AgentAvailabilityOffline = "offline"
)

type Agent struct {
	ID                    uuid.UUID       `json:"id"`
	AccountID             uuid.UUID       `json:"account_id"`
	Role                  string          `json:"role"`
	EndpointURL           string          `json:"endpoint_url"`
	CapabilitiesOffered   json.RawMessage `json:"capabilities_offered"`
	Availability          string          `json:"availability"`
	IsVerified            bool            `json:"is_verified"`
	SchemaComplianceRate  *float32        `json:"schema_compliance_rate,omitempty"`
	AvgResponseTimeMs     *int            `json:"avg_response_time_ms,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}
