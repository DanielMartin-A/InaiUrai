package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/google/uuid"
)

type ContextRepo struct{ db *sql.DB }

func NewContextRepo(db *sql.DB) *ContextRepo { return &ContextRepo{db: db} }

func (r *ContextRepo) GetSelective(ctx context.Context, orgID uuid.UUID, types []string, entityNames []string, maxTokens int) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, t := range types {
		if t == "soul" {
			t = "org_soul"
		}
		row := r.db.QueryRowContext(ctx,
			`SELECT content FROM org_context WHERE org_id = $1 AND context_type = $2 LIMIT 1`, orgID, t)
		var content json.RawMessage
		if err := row.Scan(&content); err == nil {
			result[t] = content
		}
	}
	return result, nil
}

func (r *ContextRepo) GetOrgSoul(ctx context.Context, orgID uuid.UUID) (string, error) {
	var content json.RawMessage
	err := r.db.QueryRowContext(ctx,
		`SELECT content FROM org_context WHERE org_id = $1 AND context_type = 'org_soul'`, orgID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var soul string
	if err := json.Unmarshal(content, &soul); err != nil {
		return string(content), nil
	}
	return soul, nil
}

func (r *ContextRepo) GetMemberProfile(ctx context.Context, memberID uuid.UUID) (string, error) {
	var content json.RawMessage
	err := r.db.QueryRowContext(ctx,
		`SELECT content FROM member_profiles WHERE member_id = $1 AND profile_type = 'preferences' LIMIT 1`,
		memberID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (r *ContextRepo) UpsertOrgSoul(ctx context.Context, orgID uuid.UUID, soul string) error {
	soulJSON, _ := json.Marshal(soul)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO org_context (org_id, context_type, content, source)
		 VALUES ($1, 'org_soul', $2, 'auto_generated')
		 ON CONFLICT (org_id, context_type) WHERE context_type IN ('org_soul','business_profile','known_entities','preferences')
		 DO UPDATE SET content = EXCLUDED.content, source = EXCLUDED.source`,
		orgID, soulJSON)
	return err
}

var _ = models.OrgContext{}
