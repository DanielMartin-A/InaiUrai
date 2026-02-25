package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

type AgentRepo struct {
	pool *pgxpool.Pool
}

func NewAgentRepo(pool *pgxpool.Pool) *AgentRepo {
	return &AgentRepo{pool: pool}
}

// listAgentsWhere excludes accounts where is_system_account = TRUE.
const listAgentsWhere = `FROM agents a INNER JOIN accounts ac ON ac.id = a.account_id WHERE ac.is_system_account = FALSE`

func (r *AgentRepo) Create(ctx context.Context, ag *models.Agent) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO agents (id, account_id, role, endpoint_url, capabilities_offered, availability, is_verified, schema_compliance_rate, avg_response_time_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`, ag.ID, ag.AccountID, ag.Role, ag.EndpointURL, ag.CapabilitiesOffered, ag.Availability, ag.IsVerified, ag.SchemaComplianceRate, ag.AvgResponseTimeMs).Scan(&ag.CreatedAt, &ag.UpdatedAt)
}

func (r *AgentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var ag models.Agent
	err := r.pool.QueryRow(ctx, `
		SELECT id, account_id, role, endpoint_url, capabilities_offered, availability, is_verified, schema_compliance_rate, avg_response_time_ms, created_at, updated_at
		FROM agents WHERE id = $1
	`, id).Scan(&ag.ID, &ag.AccountID, &ag.Role, &ag.EndpointURL, &ag.CapabilitiesOffered, &ag.Availability, &ag.IsVerified, &ag.SchemaComplianceRate, &ag.AvgResponseTimeMs, &ag.CreatedAt, &ag.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ag, nil
}

func (r *AgentRepo) Update(ctx context.Context, ag *models.Agent) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE agents SET account_id = $2, role = $3, endpoint_url = $4, capabilities_offered = $5, availability = $6, is_verified = $7, schema_compliance_rate = $8, avg_response_time_ms = $9, updated_at = now()
		WHERE id = $1
	`, ag.ID, ag.AccountID, ag.Role, ag.EndpointURL, ag.CapabilitiesOffered, ag.Availability, ag.IsVerified, ag.SchemaComplianceRate, ag.AvgResponseTimeMs)
	return err
}

func (r *AgentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	return err
}

// List returns all agents whose account is not a system account.
func (r *AgentRepo) List(ctx context.Context) ([]*models.Agent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.account_id, a.role, a.endpoint_url, a.capabilities_offered, a.availability, a.is_verified, a.schema_compliance_rate, a.avg_response_time_ms, a.created_at, a.updated_at
		`+listAgentsWhere+`
		ORDER BY a.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Agent
	for rows.Next() {
		var ag models.Agent
		if err := rows.Scan(&ag.ID, &ag.AccountID, &ag.Role, &ag.EndpointURL, &ag.CapabilitiesOffered, &ag.Availability, &ag.IsVerified, &ag.SchemaComplianceRate, &ag.AvgResponseTimeMs, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &ag)
	}
	return list, rows.Err()
}

// ListByAccountID returns agents for the given account, excluding system accounts.
func (r *AgentRepo) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Agent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.account_id, a.role, a.endpoint_url, a.capabilities_offered, a.availability, a.is_verified, a.schema_compliance_rate, a.avg_response_time_ms, a.created_at, a.updated_at
		`+listAgentsWhere+`
		AND a.account_id = $1
		ORDER BY a.created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Agent
	for rows.Next() {
		var ag models.Agent
		if err := rows.Scan(&ag.ID, &ag.AccountID, &ag.Role, &ag.EndpointURL, &ag.CapabilitiesOffered, &ag.Availability, &ag.IsVerified, &ag.SchemaComplianceRate, &ag.AvgResponseTimeMs, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &ag)
	}
	return list, rows.Err()
}

// FindAvailableWorkers returns agents that are workers (role = 'worker' or 'both'), online, and whose account is not a system account.
func (r *AgentRepo) FindAvailableWorkers(ctx context.Context, capability string) ([]*models.Agent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.account_id, a.role, a.endpoint_url, a.capabilities_offered, a.availability, a.is_verified, a.schema_compliance_rate, a.avg_response_time_ms, a.created_at, a.updated_at
		`+listAgentsWhere+`
		AND a.role IN ('worker', 'both') AND a.availability = 'online'
		AND (a.capabilities_offered IS NULL OR a.capabilities_offered ? $1)
		ORDER BY a.created_at DESC
	`, capability)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Agent
	for rows.Next() {
		var ag models.Agent
		if err := rows.Scan(&ag.ID, &ag.AccountID, &ag.Role, &ag.EndpointURL, &ag.CapabilitiesOffered, &ag.Availability, &ag.IsVerified, &ag.SchemaComplianceRate, &ag.AvgResponseTimeMs, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &ag)
	}
	return list, rows.Err()
}
