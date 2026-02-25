package registry

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentProfile struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	Name            string
	Slug           string
	Description     string
	Capabilities    []string
	Status          string
	BasePriceCents  int32
	WebhookURL      string
	InputSchema     json.RawMessage
	OutputSchema    json.RawMessage
	ReputationScore float64
	TotalJobs       int32
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type CreateParams struct {
	AccountID      uuid.UUID
	Name           string
	Slug           string
	Description    string
	Capabilities   []string
	BasePriceCents int32
	WebhookURL     string
	InputSchema    json.RawMessage
	OutputSchema   json.RawMessage
}

func (r *Repository) Create(ctx context.Context, p CreateParams) (*AgentProfile, error) {
	var id uuid.UUID
	var status string
	var reputationScore float64
	var totalJobs int32
	row := r.pool.QueryRow(ctx, `
		INSERT INTO agent_profiles (
			account_id, name, slug, description, capabilities, status,
			base_price_cents, webhook_url, input_schema, output_schema,
			reputation_score, total_jobs
		) VALUES ($1, $2, $3, $4, $5, 'DRAFT', $6, $7, $8, $9, 0.00, 0)
		RETURNING id, status, reputation_score, total_jobs
	`, p.AccountID, p.Name, p.Slug, p.Description, p.Capabilities,
		p.BasePriceCents, p.WebhookURL, p.InputSchema, p.OutputSchema)
	if err := row.Scan(&id, &status, &reputationScore, &totalJobs); err != nil {
		return nil, err
	}
	return &AgentProfile{
		ID:              id,
		AccountID:       p.AccountID,
		Name:            p.Name,
		Slug:            p.Slug,
		Description:     p.Description,
		Capabilities:    p.Capabilities,
		Status:          status,
		BasePriceCents:  p.BasePriceCents,
		WebhookURL:      p.WebhookURL,
		InputSchema:     p.InputSchema,
		OutputSchema:    p.OutputSchema,
		ReputationScore: reputationScore,
		TotalJobs:       totalJobs,
	}, nil
}

func (r *Repository) ListActive(ctx context.Context) ([]*AgentProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.account_id, a.role, a.endpoint_url,
		       a.capabilities_offered, a.availability,
		       a.schema_compliance_rate, a.avg_response_time_ms
		FROM agents a
		WHERE a.availability = 'online'
		ORDER BY a.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*AgentProfile
	for rows.Next() {
		var (
			id          uuid.UUID
			accountID   uuid.UUID
			role        string
			endpointURL *string
			capsJSON    json.RawMessage
			avail       *string
			compliance  *float32
			respTime    *int
		)
		if err := rows.Scan(&id, &accountID, &role, &endpointURL, &capsJSON, &avail, &compliance, &respTime); err != nil {
			return nil, err
		}
		var caps []string
		if len(capsJSON) > 0 {
			var m map[string]json.RawMessage
			if json.Unmarshal(capsJSON, &m) == nil {
				for k := range m {
					caps = append(caps, k)
				}
			}
		}
		name := id.String()[:8]
		if endpointURL != nil {
			name = *endpointURL
		}
		a := &AgentProfile{
			ID:           id,
			AccountID:    accountID,
			Name:         name,
			Slug:         id.String(),
			Description:  role + " agent",
			Capabilities: caps,
			Status:       "ACTIVE",
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
