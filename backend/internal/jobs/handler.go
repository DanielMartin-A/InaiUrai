package jobs

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/inaiurai/backend/internal/auth"
	"github.com/inaiurai/backend/internal/ledger"
)

// Request/response structs match openapi.json (snake_case JSON).

type CreateJobRequest struct {
	Title                string          `json:"title"`
	Description           string          `json:"description"`
	RequiredCapabilities  []string        `json:"required_capabilities"`
	BudgetCents           int64           `json:"budget_cents"`
	InputPayload          json.RawMessage `json:"input_payload"`
}

type JobResponse struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	BudgetCents     int64           `json:"budget_cents"`
	AssignedAgentID *string         `json:"assigned_agent_id,omitempty"`
	InputPayload    json.RawMessage `json:"input_payload"`
	OutputPayload   json.RawMessage `json:"output_payload,omitempty"`
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

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requesterID, err := h.accountIDFromRequest(r)
	if err != nil || requesterID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Title == "" || req.Description == "" || len(req.RequiredCapabilities) == 0 || req.BudgetCents <= 0 || req.InputPayload == nil {
		http.Error(w, "missing or invalid required fields", http.StatusBadRequest)
		return
	}
	job, err := h.svc.CreateJob(r.Context(), requesterID, req.Title, req.Description, req.RequiredCapabilities, req.BudgetCents, req.InputPayload)
	if err != nil {
		h.log.Error("create job failed", "error", err)
		http.Error(w, "create job failed", http.StatusInternalServerError)
		return
	}
	// Trigger matchmaker: assign agent and enqueue execution if possible
	if assignErr := h.svc.AssignAgent(r.Context(), job.ID); assignErr != nil {
		if !errors.Is(assignErr, ledger.ErrInsufficientFunds) {
			h.log.Error("assign agent failed", "error", assignErr)
		}
	}
	// Return current job state (ASSIGNED if matchmaker succeeded, else OPEN)
	if updated, _ := h.svc.GetJob(r.Context(), job.ID); updated != nil {
		job = updated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(jobToResponse(job))
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requesterID, err := h.accountIDFromRequest(r)
	if err != nil || requesterID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	list, err := h.svc.ListByRequester(r.Context(), requesterID)
	if err != nil {
		h.log.Error("list jobs failed", "error", err)
		http.Error(w, "list jobs failed", http.StatusInternalServerError)
		return
	}
	resp := make([]JobResponse, 0, len(list))
	for _, j := range list {
		resp = append(resp, jobToResponse(j))
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

func jobToResponse(j *Job) JobResponse {
	out := JobResponse{
		ID:           j.ID.String(),
		Title:        j.Title,
		Status:       j.Status,
		BudgetCents:  j.BudgetCents,
		InputPayload: j.InputPayload,
		OutputPayload: j.OutputPayload,
	}
	if j.AssignedAgentID != nil {
		s := j.AssignedAgentID.String()
		out.AssignedAgentID = &s
	}
	return out
}
