package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

type AuditRepo struct{ db *sql.DB }

func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }

type AuditEntry struct {
	StepNumber int             `json:"step_number"`
	ActionType string          `json:"action_type"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	ToolOutput json.RawMessage `json:"tool_output,omitempty"`
	TokensUsed int             `json:"tokens_used"`
	BlockedBy  string          `json:"blocked_by,omitempty"`
}

func (r *AuditRepo) StoreBatch(ctx context.Context, taskID string, orgID string, entries []AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO agent_audit_trail (task_id, org_id, step_number, action_type, tool_name, tool_input, tool_output, tokens_used, blocked_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	tID, _ := uuid.Parse(taskID)
	oID, _ := uuid.Parse(orgID)
	for _, e := range entries {
		inputJSON, _ := json.Marshal(e.ToolInput)
		outputJSON, _ := json.Marshal(e.ToolOutput)
		_, err := stmt.ExecContext(ctx, tID, oID, e.StepNumber, e.ActionType, e.ToolName, inputJSON, outputJSON, e.TokensUsed, e.BlockedBy)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *AuditRepo) GetByTaskID(ctx context.Context, taskID string) ([]AuditEntry, error) {
	tID, _ := uuid.Parse(taskID)
	rows, err := r.db.QueryContext(ctx,
		`SELECT step_number, action_type, COALESCE(tool_name,''), tool_input, tool_output, tokens_used, COALESCE(blocked_by,'')
		 FROM agent_audit_trail WHERE task_id = $1 ORDER BY step_number`, tID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		e := AuditEntry{}
		if err := rows.Scan(&e.StepNumber, &e.ActionType, &e.ToolName, &e.ToolInput, &e.ToolOutput, &e.TokensUsed, &e.BlockedBy); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
