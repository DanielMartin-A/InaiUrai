package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/google/uuid"
)

type TaskRepo struct{ db *sql.DB }

func NewTaskRepo(db *sql.DB) *TaskRepo { return &TaskRepo{db: db} }

func (r *TaskRepo) Create(ctx context.Context, t *models.Task) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO tasks (org_id, member_id, engagement_id, role_slug, input_text, status, capability)
		 VALUES ($1, $2, $3, $4, $5, 'created', 'assistant') RETURNING id, created_at`,
		t.OrgID, t.MemberID, t.EngagementID, t.RoleSlug, t.InputText,
	).Scan(&t.ID, &t.CreatedAt)
}

func (r *TaskRepo) Complete(ctx context.Context, id uuid.UUID, output string, score float64, entities json.RawMessage, processingMs int) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status='completed', output_text=$1, quality_score=$2,
		 extracted_entities=$3, processing_time_ms=$4, completed_at=$5 WHERE id=$6`,
		output, score, entities, processingMs, now, id)
	return err
}

func (r *TaskRepo) Fail(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status='failed', error_message=$1 WHERE id=$2`, errMsg, id)
	return err
}

func (r *TaskRepo) ListByEngagement(ctx context.Context, engID uuid.UUID) ([]models.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, role_slug, COALESCE(input_text,''), COALESCE(output_text,''), status,
		 quality_score, processing_time_ms, created_at, completed_at
		 FROM tasks WHERE engagement_id = $1 ORDER BY created_at`, engID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t := models.Task{}
		if err := rows.Scan(&t.ID, &t.RoleSlug, &t.InputText, &t.OutputText, &t.Status,
			&t.QualityScore, &t.ProcessingTimeMs, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *TaskRepo) CheckoutNext(ctx context.Context, engID, workerID uuid.UUID) (*models.Task, error) {
	t := &models.Task{}
	err := r.db.QueryRowContext(ctx,
		`UPDATE tasks SET checked_out_by=$1, status='processing'
		 WHERE id = (SELECT id FROM tasks WHERE engagement_id=$2 AND status='created' AND checked_out_by IS NULL
		 ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING id, role_slug, input_text`,
		workerID, engID).Scan(&t.ID, &t.RoleSlug, &t.InputText)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}
