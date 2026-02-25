package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/inaiurai/backend/internal/middleware"
	"github.com/inaiurai/backend/internal/models"
	"github.com/inaiurai/backend/internal/services"
)

// TaskRepoForHandler is the subset of task repository needed by the handler.
type TaskRepoForHandler interface {
	Create(ctx context.Context, t *models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Update(ctx context.Context, t *models.Task) error
}

// AgentRepoForHandler resolves agent → account.
type AgentRepoForHandler interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
}

// TxBeginner abstracts transaction creation so tests don't need a pgxpool.Pool.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Escrow abstracts the escrow operations needed by the handler.
type Escrow interface {
	LockCredits(ctx context.Context, tx pgx.Tx, accountID, taskID uuid.UUID, amount int) error
	SettleTask(ctx context.Context, tx pgx.Tx, taskID, requesterID, workerID uuid.UUID, budget, actualCost int) error
	RefundFailed(ctx context.Context, tx pgx.Tx, taskID, requesterID uuid.UUID, budget int) error
}

// TaskDispatcher abstracts asynchronous task dispatch.
type TaskDispatcher interface {
	DispatchTask(ctx context.Context, task *models.Task) error
}

// TaskHandler serves /v1/tasks endpoints.
type TaskHandler struct {
	Pool       TxBeginner
	TaskRepo   TaskRepoForHandler
	AgentRepo  AgentRepoForHandler
	Escrow     Escrow
	Dispatcher TaskDispatcher
	Validator  *services.Validator
	Logger     *slog.Logger
}

// --- POST /v1/tasks ---

type createTaskRequest struct {
	RequesterAgentID  string          `json:"requester_agent_id"`
	CapabilityRequired string         `json:"capability_required"`
	InputPayload      json.RawMessage `json:"input_payload"`
	Budget            int             `json:"budget"`
	RoutingPreference string          `json:"routing_preference"`
}

type createTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// CreateTask handles POST /v1/tasks.
// Auth -> Budget (via middleware) -> Validate Input -> Lock Credits -> Dispatch Async -> 202.
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	acc := middleware.AccountFromCtx(r.Context())
	if acc == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	requesterAgentID, err := uuid.Parse(req.RequesterAgentID)
	if err != nil {
		http.Error(w, `{"error":"invalid requester_agent_id"}`, http.StatusBadRequest)
		return
	}
	if req.CapabilityRequired == "" {
		http.Error(w, `{"error":"capability_required is required"}`, http.StatusBadRequest)
		return
	}
	if req.Budget <= 0 {
		http.Error(w, `{"error":"budget must be > 0"}`, http.StatusBadRequest)
		return
	}

	// Validate input payload against capability schema (hard reject).
	if err := h.Validator.ValidateInput(r.Context(), req.CapabilityRequired, req.InputPayload); err != nil {
		if errors.Is(err, services.ErrValidation) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		h.Logger.Error("validate input", "error", err)
		http.Error(w, `{"error":"input validation failed"}`, http.StatusBadRequest)
		return
	}

	task := &models.Task{
		ID:                 uuid.New(),
		RequesterAgentID:   requesterAgentID,
		CapabilityRequired: req.CapabilityRequired,
		InputPayload:       req.InputPayload,
		Budget:             req.Budget,
		Status:             models.TaskStatusCreated,
		RoutingPreference:  req.RoutingPreference,
	}

	// Lock credits in a transaction, then persist the task.
	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		h.Logger.Error("begin tx", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	if err := h.Escrow.LockCredits(r.Context(), tx, acc.ID, task.ID, task.Budget); err != nil {
		if errors.Is(err, services.ErrInsufficientFunds) {
			http.Error(w, `{"error":"insufficient funds"}`, http.StatusPaymentRequired)
			return
		}
		h.Logger.Error("lock credits", "error", err)
		http.Error(w, `{"error":"failed to lock credits"}`, http.StatusInternalServerError)
		return
	}

	task.Status = models.TaskStatusMatching
	if err := h.TaskRepo.Create(r.Context(), task); err != nil {
		h.Logger.Error("create task", "error", err)
		http.Error(w, `{"error":"failed to create task"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.Logger.Error("commit tx", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Dispatch asynchronously — don't block the API response.
	go func() {
		if err := h.Dispatcher.DispatchTask(r.Context(), task); err != nil {
			h.Logger.Error("async dispatch failed", "task_id", task.ID, "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, createTaskResponse{
		TaskID: task.ID.String(),
		Status: task.Status,
	})
}

// --- POST /v1/tasks/{id}/result ---

type taskResultRequest struct {
	OutputPayload json.RawMessage `json:"output_payload"`
	OutputStatus  string          `json:"output_status"`
	ActualCost    int             `json:"actual_cost"`
}

// SubmitResult handles POST /v1/tasks/{id}/result — the worker callback.
// Validate Output (soft) -> Settle Payment -> Update Stats -> 200.
func (h *TaskHandler) SubmitResult(w http.ResponseWriter, r *http.Request) {
	taskID, ok := extractTaskID(r)
	if !ok {
		http.Error(w, `{"error":"invalid task id"}`, http.StatusBadRequest)
		return
	}

	task, err := h.TaskRepo.GetByID(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	if task.Status != models.TaskStatusInProgress && task.Status != models.TaskStatusDispatched {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "task is not in a dispatchable state", "status": task.Status})
		return
	}

	callingAgent := middleware.AgentFromCtx(r.Context())
	if callingAgent == nil || task.WorkerAgentID == nil || callingAgent.ID != *task.WorkerAgentID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "caller is not the assigned worker"})
		return
	}

	var req taskResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.OutputStatus == "" {
		req.OutputStatus = models.TaskOutputStatusSuccess
	}

	// Soft validate output — log but don't reject.
	if valErr := h.Validator.ValidateOutput(r.Context(), task.CapabilityRequired, req.OutputPayload); valErr != nil {
		h.Logger.Warn("output validation failed (soft flag)", "task_id", taskID, "error", valErr)
	}

	task.OutputPayload = req.OutputPayload
	task.OutputStatus = req.OutputStatus
	task.ActualCost = &req.ActualCost

	// Settle payment.
	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		h.Logger.Error("begin settle tx", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	requesterAccount, err := h.resolveAccountID(r.Context(), task.RequesterAgentID)
	if err != nil {
		h.Logger.Error("resolve requester account", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	switch req.OutputStatus {
	case models.TaskOutputStatusSuccess, models.TaskOutputStatusPartial:
		if task.WorkerAgentID == nil {
			h.Logger.Error("task has no worker_agent_id", "task_id", taskID)
			http.Error(w, `{"error":"no worker assigned"}`, http.StatusInternalServerError)
			return
		}
		workerAccount, err := h.resolveAccountID(r.Context(), *task.WorkerAgentID)
		if err != nil {
			h.Logger.Error("resolve worker account", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		actualCost := req.ActualCost
		if actualCost <= 0 {
			actualCost = task.Budget
		}
		platformFee := actualCost * 10 / 100
		task.PlatformFee = &platformFee
		task.Status = models.TaskStatusCompleted

		if err := h.Escrow.SettleTask(r.Context(), tx, task.ID, requesterAccount, workerAccount, task.Budget, actualCost); err != nil {
			h.Logger.Error("settle task", "error", err)
			http.Error(w, `{"error":"settlement failed"}`, http.StatusInternalServerError)
			return
		}

	case models.TaskOutputStatusError:
		task.Status = models.TaskStatusFailed
		if err := h.Escrow.RefundFailed(r.Context(), tx, task.ID, requesterAccount, task.Budget); err != nil {
			h.Logger.Error("refund failed", "error", err)
			http.Error(w, `{"error":"refund failed"}`, http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, `{"error":"invalid output_status"}`, http.StatusBadRequest)
		return
	}

	if err := h.TaskRepo.Update(r.Context(), task); err != nil {
		h.Logger.Error("update task after settle", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.Logger.Error("commit settle tx", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Update worker stats (avg_response_time_ms, schema_compliance_rate) asynchronously.
	if task.WorkerAgentID != nil {
		go h.updateWorkerStats(task)
	}

	writeJSON(w, http.StatusOK, map[string]string{"task_id": task.ID.String(), "status": task.Status})
}

// --- GET /v1/tasks/{id} ---

// GetTask handles GET /v1/tasks/{id}.
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID, ok := extractTaskID(r)
	if !ok {
		http.Error(w, `{"error":"invalid task id"}`, http.StatusBadRequest)
		return
	}

	task, err := h.TaskRepo.GetByID(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// --- GET /v1/capabilities ---

type capabilityInfo struct {
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Deadline string `json:"deadline"`
}

// ListCapabilities handles GET /v1/capabilities (public, no auth).
func ListCapabilities(w http.ResponseWriter, _ *http.Request) {
	caps := []capabilityInfo{
		{Name: "research", Price: 8, Deadline: "15s–45s (depth-dependent)"},
		{Name: "summarize", Price: 3, Deadline: "15s"},
		{Name: "data_extraction", Price: 5, Deadline: "20s"},
	}
	writeJSON(w, http.StatusOK, caps)
}

// --- helpers ---

func (h *TaskHandler) resolveAccountID(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error) {
	agent, err := h.AgentRepo.GetByID(ctx, agentID)
	if err != nil {
		return uuid.Nil, err
	}
	return agent.AccountID, nil
}

func (h *TaskHandler) updateWorkerStats(task *models.Task) {
	// Placeholder: update schema_compliance_rate and avg_response_time_ms on the worker agent.
	// This would compute new averages from completed tasks. For MVP, log only.
	h.Logger.Info("worker stats update", "worker_agent_id", task.WorkerAgentID, "task_id", task.ID, "output_status", task.OutputStatus)
}

// extractTaskID parses the task UUID from the URL path.
// Supports paths like /v1/tasks/{id} and /v1/tasks/{id}/result.
func extractTaskID(r *http.Request) (uuid.UUID, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	// path is now "{id}" or "{id}/result"
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
