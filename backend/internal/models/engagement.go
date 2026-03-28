package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Engagement struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	CreatedBy          *uuid.UUID      `json:"created_by,omitempty"`
	Name               string          `json:"name,omitempty"`
	Objective          string          `json:"objective"`
	EngagementType     string          `json:"engagement_type"`
	Status             string          `json:"status"`
	Roles              json.RawMessage `json:"roles"`
	ExecutionPlan      json.RawMessage `json:"execution_plan,omitempty"`
	BudgetMonthlyCents *int            `json:"budget_monthly_cents,omitempty"`
	BudgetSpentCents   int             `json:"budget_spent_cents"`
	HeartbeatConfig    json.RawMessage `json:"heartbeat_config,omitempty"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type EngagementRole struct {
	RoleSlug           string `json:"role_slug"`
	Purpose            string `json:"purpose"`
	MonthlyBudgetCents int    `json:"monthly_budget_cents,omitempty"`
}
