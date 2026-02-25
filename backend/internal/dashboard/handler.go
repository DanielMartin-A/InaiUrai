package dashboard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/inaiurai/backend/internal/auth"
	"github.com/inaiurai/backend/internal/models"
	"github.com/inaiurai/backend/internal/repository"
)

type Handler struct {
	authSvc    auth.Service
	accountR   *repository.AccountRepo
	creditR    *repository.CreditRepo
	apiKeyR    *repository.APIKeyRepo
	taskR      *repository.TaskRepo
	agentR     *repository.AgentRepo
	log        *slog.Logger
}

func NewHandler(
	authSvc auth.Service,
	accountR *repository.AccountRepo,
	creditR *repository.CreditRepo,
	apiKeyR *repository.APIKeyRepo,
	taskR *repository.TaskRepo,
	agentR *repository.AgentRepo,
	log *slog.Logger,
) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		authSvc:  authSvc,
		accountR: accountR,
		creditR:  creditR,
		apiKeyR:  apiKeyR,
		taskR:    taskR,
		agentR:   agentR,
		log:      log,
	}
}

func (h *Handler) accountIDFromRequest(r *http.Request) (uuid.UUID, error) {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return uuid.Nil, fmt.Errorf("missing authorization")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return uuid.Nil, fmt.Errorf("bad authorization format")
	}
	token := strings.TrimSpace(authz[len(prefix):])
	if token == "" {
		return uuid.Nil, fmt.Errorf("empty token")
	}
	id, _, err := h.authSvc.ValidateToken(r.Context(), token)
	return id, err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GET /api/v1/account/me
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	acc, err := h.accountR.GetByID(r.Context(), accountID)
	if err != nil {
		h.log.Error("get account failed", "error", err)
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                acc.ID,
		"email":             acc.Email,
		"display_name":      acc.Name,
		"company":           acc.Company,
		"balance_cents":     acc.CreditBalance,
		"subscription_tier": acc.SubscriptionTier,
		"global_max_per_task": acc.GlobalMaxPerTask,
		"global_max_per_day":  acc.GlobalMaxPerDay,
		"created_at":        acc.CreatedAt,
	})
}

// PATCH /api/v1/account/settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	acc, err := h.accountR.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	var body struct {
		Name             *string `json:"display_name"`
		Company          *string `json:"company"`
		Email            *string `json:"email"`
		GlobalMaxPerTask *int    `json:"global_max_per_task"`
		GlobalMaxPerDay  *int    `json:"global_max_per_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		acc.Name = *body.Name
	}
	if body.Company != nil {
		acc.Company = *body.Company
	}
	if body.Email != nil {
		acc.Email = *body.Email
	}
	if body.GlobalMaxPerTask != nil {
		acc.GlobalMaxPerTask = body.GlobalMaxPerTask
	}
	if body.GlobalMaxPerDay != nil {
		acc.GlobalMaxPerDay = body.GlobalMaxPerDay
	}
	if err := h.accountR.Update(r.Context(), acc); err != nil {
		h.log.Error("update settings failed", "error", err)
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/credit-ledger
func (h *Handler) ListCreditLedger(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	entries, err := h.creditR.ListByAccountID(r.Context(), accountID)
	if err != nil {
		h.log.Error("list credit ledger failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []*models.CreditLedger{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// GET /api/v1/api-keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	keys, err := h.apiKeyR.ListByAccountID(r.Context(), accountID)
	if err != nil {
		h.log.Error("list api keys failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

// POST /api/v1/api-keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		http.Error(w, "key generation failed", http.StatusInternalServerError)
		return
	}
	rawKey := "inai_" + hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	keyPrefix := rawKey[:12]

	k := &models.APIKey{
		ID:        uuid.New(),
		AccountID: accountID,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		IsActive:  true,
	}
	if err := h.apiKeyR.Create(r.Context(), k); err != nil {
		h.log.Error("create api key failed", "error", err)
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         k.ID,
		"key_prefix": k.KeyPrefix,
		"is_active":  k.IsActive,
		"raw_key":    rawKey,
	})
}

// DELETE /api/v1/api-keys/{id}
func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	_, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := r.URL.Path
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	idStr := parts[len(parts)-1]
	keyID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid key ID", http.StatusBadRequest)
		return
	}
	if err := h.apiKeyR.Delete(r.Context(), keyID); err != nil {
		h.log.Error("delete api key failed", "error", err)
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/tasks
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	_, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tasks, err := h.taskR.List(r.Context())
	if err != nil {
		h.log.Error("list tasks failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// POST /api/v1/agents/kill-all
func (h *Handler) KillAllAgents(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	agents, err := h.agentR.ListByAccountID(r.Context(), accountID)
	if err != nil {
		h.log.Error("list agents for kill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for _, ag := range agents {
		ag.Availability = "offline"
		if err := h.agentR.Update(r.Context(), ag); err != nil {
			h.log.Error("kill agent failed", "agent_id", ag.ID, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/agents/resume-all
func (h *Handler) ResumeAllAgents(w http.ResponseWriter, r *http.Request) {
	accountID, err := h.accountIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	agents, err := h.agentR.ListByAccountID(r.Context(), accountID)
	if err != nil {
		h.log.Error("list agents for resume failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for _, ag := range agents {
		ag.Availability = "online"
		if err := h.agentR.Update(r.Context(), ag); err != nil {
			h.log.Error("resume agent failed", "agent_id", ag.ID, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
