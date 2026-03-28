package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type CostRepo struct{ db *sql.DB }

func NewCostRepo(db *sql.DB) *CostRepo { return &CostRepo{db: db} }

func (r *CostRepo) GetDaily(ctx context.Context, orgID uuid.UUID) (int, int, int, error) {
	var tokens, toolCalls, costCents int
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_tokens),0), COALESCE(SUM(total_tool_calls),0), COALESCE(SUM(estimated_cost_cents),0)
		 FROM daily_cost_tracking WHERE customer_id IN (SELECT id FROM members WHERE org_id = $1) AND date = CURRENT_DATE`,
		orgID).Scan(&tokens, &toolCalls, &costCents)
	return tokens, toolCalls, costCents, err
}

func (r *CostRepo) Record(ctx context.Context, orgID uuid.UUID, tokens, toolCalls, costCents int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO daily_cost_tracking (customer_id, date, total_tokens, total_tool_calls, estimated_cost_cents, task_count)
		 SELECT m.id, CURRENT_DATE, $2, $3, $4, 1 FROM members m WHERE m.org_id = $1 AND m.is_admin = TRUE LIMIT 1
		 ON CONFLICT (customer_id, date) DO UPDATE SET
		   total_tokens = daily_cost_tracking.total_tokens + EXCLUDED.total_tokens,
		   total_tool_calls = daily_cost_tracking.total_tool_calls + EXCLUDED.total_tool_calls,
		   estimated_cost_cents = daily_cost_tracking.estimated_cost_cents + EXCLUDED.estimated_cost_cents,
		   task_count = daily_cost_tracking.task_count + 1`,
		orgID, tokens, toolCalls, costCents)
	return err
}
