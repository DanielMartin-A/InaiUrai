package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/google/uuid"
)

type EngagementRepo struct{ db *sql.DB }

func NewEngagementRepo(db *sql.DB) *EngagementRepo { return &EngagementRepo{db: db} }

func (r *EngagementRepo) Create(ctx context.Context, e *models.Engagement) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO engagements (org_id, created_by, name, objective, engagement_type, status, roles, execution_plan, heartbeat_config)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, created_at`,
		e.OrgID, e.CreatedBy, e.Name, e.Objective, e.EngagementType, e.Status, e.Roles, e.ExecutionPlan, e.HeartbeatConfig,
	).Scan(&e.ID, &e.CreatedAt)
}

func (r *EngagementRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Engagement, error) {
	e := &models.Engagement{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, created_by, COALESCE(name,''), objective, engagement_type, status,
		 roles, execution_plan, budget_monthly_cents, budget_spent_cents, heartbeat_config,
		 started_at, completed_at, created_at
		 FROM engagements WHERE id = $1`, id).Scan(
		&e.ID, &e.OrgID, &e.CreatedBy, &e.Name, &e.Objective, &e.EngagementType, &e.Status,
		&e.Roles, &e.ExecutionPlan, &e.BudgetMonthlyCents, &e.BudgetSpentCents,
		&e.HeartbeatConfig, &e.StartedAt, &e.CompletedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

func (r *EngagementRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, status string) ([]models.Engagement, error) {
	query := `SELECT id, org_id, COALESCE(name,''), objective, engagement_type, status, roles, created_at
		FROM engagements WHERE org_id = $1`
	args := []interface{}{orgID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 50"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Engagement
	for rows.Next() {
		e := models.Engagement{}
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Name, &e.Objective, &e.EngagementType, &e.Status, &e.Roles, &e.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *EngagementRepo) GetActiveWithHeartbeats(ctx context.Context) ([]models.Engagement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, objective, engagement_type, roles, heartbeat_config
		 FROM engagements WHERE status = 'active' AND heartbeat_config IS NOT NULL AND heartbeat_config != 'null'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Engagement
	for rows.Next() {
		e := models.Engagement{}
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Objective, &e.EngagementType, &e.Roles, &e.HeartbeatConfig); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *EngagementRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE engagements SET status = $1 WHERE id = $2`, status, id)
	return err
}

func ParseRoles(raw json.RawMessage) ([]models.EngagementRole, error) {
	var roles []models.EngagementRole
	if err := json.Unmarshal(raw, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}
