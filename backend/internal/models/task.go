package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID               uuid.UUID       `json:"id"`
	OrgID            *uuid.UUID      `json:"org_id,omitempty"`
	MemberID         *uuid.UUID      `json:"member_id,omitempty"`
	EngagementID     *uuid.UUID      `json:"engagement_id,omitempty"`
	RoleSlug         string          `json:"role_slug"`
	InputText        string          `json:"input_text"`
	OutputText       string          `json:"output_text,omitempty"`
	Status           string          `json:"status"`
	QualityScore     float64         `json:"quality_score,omitempty"`
	ProcessingTimeMs int             `json:"processing_time_ms,omitempty"`
	Entities         json.RawMessage `json:"extracted_entities,omitempty"`
	ParentTaskID     *uuid.UUID      `json:"parent_task_id,omitempty"`
	DependsOn        []uuid.UUID     `json:"depends_on,omitempty"`
	CheckedOutBy     *uuid.UUID      `json:"checked_out_by,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

type EngineRequest struct {
	TaskID          string                 `json:"task_id"`
	InputText       string                 `json:"input_text"`
	OrgContext      map[string]interface{} `json:"org_context"`
	OrgSoul         string                 `json:"org_soul"`
	MemberProfile   string                 `json:"member_profile"`
	Role            string                 `json:"role"`
	Tier            string                 `json:"tier"`
	EngagementID    string                 `json:"engagement_id"`
	OrgID           string                 `json:"org_id"`
	MemberID        string                 `json:"member_id"`
	GoalAncestry    map[string]interface{} `json:"goal_ancestry,omitempty"`
	RoleBudgetCents int                    `json:"role_budget_cents"`
}

type EngineResponse struct {
	OutputText        string                 `json:"output_text"`
	QualityScore      float64                `json:"quality_score"`
	Status            string                 `json:"status"`
	ExtractedEntities map[string]interface{} `json:"extracted_entities"`
	ProcessingTimeMs  int                    `json:"processing_time_ms"`
	AuditSummary      map[string]interface{} `json:"audit_summary"`
	CostSummary       map[string]interface{} `json:"cost_summary"`
}
