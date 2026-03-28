package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/google/uuid"
)

type InternalHandler struct {
	contextRepo *repository.ContextRepo
	costRepo    *repository.CostRepo
	auditRepo   *repository.AuditRepo
	orgRepo     *repository.OrgRepo
}

func NewInternalHandler(cr *repository.ContextRepo, co *repository.CostRepo, ar *repository.AuditRepo, or *repository.OrgRepo) *InternalHandler {
	return &InternalHandler{contextRepo: cr, costRepo: co, auditRepo: ar, orgRepo: or}
}

func (h *InternalHandler) SelectiveContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID        string   `json:"org_id"`
		ContextTypes []string `json:"context_types"`
		EntityNames  []string `json:"entity_names"`
		MaxTokens    int      `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		http.Error(w, "invalid org_id", 400)
		return
	}

	result, err := h.contextRepo.GetSelective(r.Context(), orgID, req.ContextTypes, req.EntityNames, req.MaxTokens)
	if err != nil {
		slog.Error("selective context failed", "error", err, "org_id", req.OrgID)
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (h *InternalHandler) DailyCost(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.PathValue("orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "invalid org_id", 400)
		return
	}

	tokens, toolCalls, costCents, err := h.costRepo.GetDaily(r.Context(), orgID)
	if err != nil {
		slog.Error("daily cost query failed", "error", err, "org_id", orgIDStr)
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"estimated_cost_cents": costCents, "total_tokens": tokens, "total_tool_calls": toolCalls,
	})
}

func (h *InternalHandler) RecordCost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID              string `json:"org_id"`
		Tokens             int    `json:"tokens"`
		ToolCalls          int    `json:"tool_calls"`
		EstimatedCostCents int    `json:"estimated_cost_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	orgID, _ := uuid.Parse(req.OrgID)
	if err := h.costRepo.Record(r.Context(), orgID, req.Tokens, req.ToolCalls, req.EstimatedCostCents); err != nil {
		slog.Error("record cost failed", "error", err)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *InternalHandler) StoreAudit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID  string                  `json:"task_id"`
		OrgID   string                  `json:"org_id"`
		Entries []repository.AuditEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if err := h.auditRepo.StoreBatch(r.Context(), req.TaskID, req.OrgID, req.Entries); err != nil {
		slog.Error("audit store failed", "error", err, "task_id", req.TaskID)
		http.Error(w, "internal error", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *InternalHandler) GetAudit(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	entries, err := h.auditRepo.GetByTaskID(r.Context(), taskID)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
}

func (h *InternalHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemberID string `json:"member_id"`
		OrgID    string `json:"org_id"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	memberID, _ := uuid.Parse(req.MemberID)
	orgID, _ := uuid.Parse(req.OrgID)
	if memberID == uuid.Nil || orgID == uuid.Nil {
		http.Error(w, "invalid member_id or org_id", 400)
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "token generation failed", 500)
		return
	}
	plaintext := hex.EncodeToString(tokenBytes)
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(plaintext)))

	name := req.Name
	if name == "" {
		name = "default"
	}

	_, err := h.orgRepo.DB().ExecContext(r.Context(),
		`INSERT INTO api_tokens (member_id, org_id, token_hash, name) VALUES ($1, $2, $3, $4)`,
		memberID, orgID, tokenHash, name)
	if err != nil {
		slog.Error("create token failed", "error", err)
		http.Error(w, "internal error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": plaintext,
		"name":  name,
	})
}
