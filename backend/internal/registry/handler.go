package registry

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/inaiurai/backend/internal/auth"
)

// Request/response structs match openapi.json (snake_case JSON).

type CreateAgentRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Capabilities   []string        `json:"capabilities"`
	BasePriceCents int32           `json:"base_price_cents"`
	WebhookURL     string          `json:"webhook_url"`
	InputSchema    json.RawMessage `json:"input_schema"`
	OutputSchema   json.RawMessage `json:"output_schema"`
}

type AgentProfileResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Capabilities    []string        `json:"capabilities"`
	Status          string          `json:"status"`
	BasePriceCents  int32           `json:"base_price_cents"`
	InputSchema     json.RawMessage `json:"input_schema"`
	OutputSchema    json.RawMessage `json:"output_schema"`
}

type Handler struct {
	svc     Service
	authSvc auth.Service
	log     *slog.Logger
}

func NewHandler(svc Service, authSvc auth.Service, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, authSvc: authSvc, log: log}
}

func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	accountID, err := h.accountIDFromRequest(r)
	if err != nil || accountID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Description == "" || req.WebhookURL == "" || len(req.Capabilities) == 0 || req.InputSchema == nil || req.OutputSchema == nil {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	if req.BasePriceCents < 0 {
		http.Error(w, "base_price_cents must be >= 0", http.StatusBadRequest)
		return
	}
	agent, err := h.svc.CreateAgent(r.Context(), accountID, req.Name, req.Description, req.Capabilities, req.BasePriceCents, req.WebhookURL, req.InputSchema, req.OutputSchema)
	if err != nil {
		h.log.Error("create agent failed", "error", err)
		http.Error(w, "create agent failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(agentToResponse(agent))
}

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, err := h.svc.ListActiveAgents(r.Context())
	if err != nil {
		h.log.Error("list agents failed", "error", err)
		http.Error(w, "list agents failed", http.StatusInternalServerError)
		return
	}
	resp := make([]AgentProfileResponse, 0, len(list))
	for _, a := range list {
		resp = append(resp, agentToResponse(a))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) accountIDFromRequest(r *http.Request) (uuid.UUID, error) {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return uuid.Nil, nil
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return uuid.Nil, nil
	}
	token := strings.TrimSpace(authz[len(prefix):])
	if token == "" {
		return uuid.Nil, nil
	}
	id, _, err := h.authSvc.ValidateToken(r.Context(), token)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func agentToResponse(a *AgentProfile) AgentProfileResponse {
	return AgentProfileResponse{
		ID:             a.ID.String(),
		Name:           a.Name,
		Description:    a.Description,
		Capabilities:   a.Capabilities,
		Status:         a.Status,
		BasePriceCents: a.BasePriceCents,
		InputSchema:    a.InputSchema,
		OutputSchema:   a.OutputSchema,
	}
}
