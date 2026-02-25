package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

type TaskRepo struct {
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{pool: pool}
}

func (r *TaskRepo) Create(ctx context.Context, t *models.Task) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO tasks (id, requester_agent_id, worker_agent_id, capability_required, input_payload, output_payload, output_status, status, budget, actual_cost, platform_fee, deadline, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`, t.ID, t.RequesterAgentID, t.WorkerAgentID, t.CapabilityRequired, t.InputPayload, t.OutputPayload, t.OutputStatus, t.Status, t.Budget, t.ActualCost, t.PlatformFee, t.Deadline, t.RetryCount).Scan(&t.CreatedAt, &t.UpdatedAt)
}

func (r *TaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	var t models.Task
	err := r.pool.QueryRow(ctx, `
		SELECT id, requester_agent_id, worker_agent_id, capability_required, input_payload, output_payload, output_status, status, budget, actual_cost, platform_fee, deadline, retry_count, created_at, updated_at
		FROM tasks WHERE id = $1
	`, id).Scan(&t.ID, &t.RequesterAgentID, &t.WorkerAgentID, &t.CapabilityRequired, &t.InputPayload, &t.OutputPayload, &t.OutputStatus, &t.Status, &t.Budget, &t.ActualCost, &t.PlatformFee, &t.Deadline, &t.RetryCount, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepo) Update(ctx context.Context, t *models.Task) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tasks SET requester_agent_id = $2, worker_agent_id = $3, capability_required = $4, input_payload = $5, output_payload = $6, output_status = $7, status = $8, budget = $9, actual_cost = $10, platform_fee = $11, deadline = $12, retry_count = $13, updated_at = now()
		WHERE id = $1
	`, t.ID, t.RequesterAgentID, t.WorkerAgentID, t.CapabilityRequired, t.InputPayload, t.OutputPayload, t.OutputStatus, t.Status, t.Budget, t.ActualCost, t.PlatformFee, t.Deadline, t.RetryCount)
	return err
}

func (r *TaskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM tasks WHERE id = $1", id)
	return err
}

func (r *TaskRepo) List(ctx context.Context) ([]*models.Task, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, requester_agent_id, worker_agent_id, capability_required, input_payload, output_payload, output_status, status, budget, actual_cost, platform_fee, deadline, retry_count, created_at, updated_at
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.RequesterAgentID, &t.WorkerAgentID, &t.CapabilityRequired, &t.InputPayload, &t.OutputPayload, &t.OutputStatus, &t.Status, &t.Budget, &t.ActualCost, &t.PlatformFee, &t.Deadline, &t.RetryCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, rows.Err()
}

func (r *TaskRepo) ListByRequesterAgentID(ctx context.Context, requesterAgentID uuid.UUID) ([]*models.Task, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, requester_agent_id, worker_agent_id, capability_required, input_payload, output_payload, output_status, status, budget, actual_cost, platform_fee, deadline, retry_count, created_at, updated_at
		FROM tasks WHERE requester_agent_id = $1 ORDER BY created_at DESC
	`, requesterAgentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.RequesterAgentID, &t.WorkerAgentID, &t.CapabilityRequired, &t.InputPayload, &t.OutputPayload, &t.OutputStatus, &t.Status, &t.Budget, &t.ActualCost, &t.PlatformFee, &t.Deadline, &t.RetryCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, rows.Err()
}
