package jobs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *Repository) Create(ctx context.Context, requesterID uuid.UUID, title, description string, capabilities []string, budgetCents int64, inputPayload json.RawMessage) (*Job, error) {
	var j Job
	row := r.pool.QueryRow(ctx, `
		INSERT INTO jobs (requester_id, title, description, required_capabilities, status, budget_cents, input_payload)
		VALUES ($1, $2, $3, $4, 'OPEN', $5, $6)
		RETURNING id, requester_id, title, description, status, budget_cents, required_capabilities, input_payload,
			assigned_agent_id, agreed_price_cents, output_payload
	`, requesterID, title, description, capabilities, budgetCents, inputPayload)
	err := row.Scan(&j.ID, &j.RequesterID, &j.Title, &j.Description, &j.Status, &j.BudgetCents, &j.RequiredCapabilities, &j.InputPayload,
		&j.AssignedAgentID, &j.AgreedPriceCents, &j.OutputPayload)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *Repository) GetByID(ctx context.Context, jobID uuid.UUID) (*Job, error) {
	var j Job
	row := r.pool.QueryRow(ctx, `
		SELECT id, requester_id, title, description, status, budget_cents, agreed_price_cents,
			required_capabilities, input_payload, assigned_agent_id, output_payload
		FROM jobs WHERE id = $1
	`, jobID)
	err := row.Scan(&j.ID, &j.RequesterID, &j.Title, &j.Description, &j.Status, &j.BudgetCents, &j.AgreedPriceCents,
		&j.RequiredCapabilities, &j.InputPayload, &j.AssignedAgentID, &j.OutputPayload)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// MatchingAgent holds the result of a matchmaker query.
type MatchingAgent struct {
	AgentID        uuid.UUID
	WebhookURL     string
	BasePriceCents int32
	AccountID      uuid.UUID
}

// FindMatchingAgent returns one ACTIVE agent that has all required capabilities and base_price_cents <= maxBudgetCents.
func (r *Repository) FindMatchingAgent(ctx context.Context, tx pgx.Tx, requiredCapabilities []string, maxBudgetCents int64) (*MatchingAgent, error) {
	var m MatchingAgent
	row := tx.QueryRow(ctx, `
		SELECT id, webhook_url, base_price_cents, account_id
		FROM agent_profiles
		WHERE status = 'ACTIVE' AND base_price_cents <= $1 AND capabilities @> $2
		ORDER BY base_price_cents ASC
		LIMIT 1
	`, maxBudgetCents, requiredCapabilities)
	if err := row.Scan(&m.AgentID, &m.WebhookURL, &m.BasePriceCents, &m.AccountID); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) UpdateAssigned(ctx context.Context, tx pgx.Tx, jobID, agentID uuid.UUID, agreedPriceCents int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE jobs SET status = 'ASSIGNED', assigned_agent_id = $1, agreed_price_cents = $2, assigned_at = now(), updated_at = now()
		WHERE id = $3
	`, agentID, agreedPriceCents, jobID)
	return err
}

func (r *Repository) ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*Job, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, requester_id, title, description, status, budget_cents, agreed_price_cents,
			required_capabilities, input_payload, assigned_agent_id, output_payload
		FROM jobs WHERE requester_id = $1 ORDER BY created_at DESC
	`, requesterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Job
	for rows.Next() {
		var j Job
		err := rows.Scan(&j.ID, &j.RequesterID, &j.Title, &j.Description, &j.Status, &j.BudgetCents, &j.AgreedPriceCents,
			&j.RequiredCapabilities, &j.InputPayload, &j.AssignedAgentID, &j.OutputPayload)
		if err != nil {
			return nil, err
		}
		list = append(list, &j)
	}
	return list, rows.Err()
}

func (r *Repository) MarkJobCompleted(ctx context.Context, jobID uuid.UUID, outputPayload json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = 'SETTLED', output_payload = $1, completed_at = now(), updated_at = now() WHERE id = $2
	`, outputPayload, jobID)
	return err
}

func (r *Repository) MarkJobFailed(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = 'FAILED', completed_at = now(), updated_at = now() WHERE id = $1
	`, jobID)
	return err
}

// GetProviderAccountID returns the account_id of the agent's owner for a given job (for ledger release).
func (r *Repository) GetProviderAccountID(ctx context.Context, jobID uuid.UUID) (uuid.UUID, error) {
	var accountID uuid.UUID
	row := r.pool.QueryRow(ctx, `
		SELECT ap.account_id FROM jobs j
		JOIN agent_profiles ap ON ap.id = j.assigned_agent_id
		WHERE j.id = $1
	`, jobID)
	if err := row.Scan(&accountID); err != nil {
		return uuid.Nil, err
	}
	return accountID, nil
}
