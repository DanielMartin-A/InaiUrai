package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

const (
	dispatchTimeout = 5 * time.Second
	maxRetries      = 2
)

// DispatcherTaskRepo is the task repository interface used by the dispatcher.
type DispatcherTaskRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Update(ctx context.Context, t *models.Task) error
}

// DispatcherAgentRepo is the agent repository interface used by the dispatcher
// to look up the requester agent's account.
type DispatcherAgentRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
}

// Dispatcher orchestrates matching, webhook delivery, deadline monitoring, and fallback.
type Dispatcher struct {
	Pool       *pgxpool.Pool
	Matcher    *Matcher
	Validator  *Validator
	Escrow     *EscrowService
	TaskRepo   DispatcherTaskRepo
	AgentRepo  DispatcherAgentRepo
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewDispatcher returns a Dispatcher with the 5-second dispatch HTTP client.
func NewDispatcher(
	pool *pgxpool.Pool,
	matcher *Matcher,
	validator *Validator,
	escrow *EscrowService,
	taskRepo DispatcherTaskRepo,
	agentRepo DispatcherAgentRepo,
	logger *slog.Logger,
) *Dispatcher {
	return &Dispatcher{
		Pool:      pool,
		Matcher:   matcher,
		Validator: validator,
		Escrow:    escrow,
		TaskRepo:  taskRepo,
		AgentRepo: agentRepo,
		HTTPClient: &http.Client{Timeout: dispatchTimeout},
		Logger:    logger,
	}
}

// dispatchPayload is the JSON body sent to the worker agent's endpoint.
type dispatchPayload struct {
	TaskID       uuid.UUID       `json:"task_id"`
	Capability   string          `json:"capability"`
	InputPayload json.RawMessage `json:"input_payload"`
	CallbackURL  string          `json:"callback_url"`
	Deadline     time.Time       `json:"deadline"`
}

// DispatchTask finds the best worker, sends the webhook, and starts deadline monitoring.
func (d *Dispatcher) DispatchTask(ctx context.Context, task *models.Task) error {
	worker, err := d.Matcher.FindBestWorker(ctx, task)
	if err != nil {
		return fmt.Errorf("find best worker: %w", err)
	}
	if worker == nil {
		return d.failNoWorker(ctx, task)
	}
	return d.dispatchToWorker(ctx, task, worker)
}

// dispatchToWorker sends the task to a specific worker and sets up deadline monitoring.
func (d *Dispatcher) dispatchToWorker(ctx context.Context, task *models.Task, worker *models.Agent) error {
	deadlineDur, err := d.Validator.GetDeadline(task.CapabilityRequired, task.InputPayload)
	if err != nil {
		return fmt.Errorf("get deadline: %w", err)
	}
	deadline := time.Now().Add(deadlineDur)

	task.WorkerAgentID = &worker.ID
	task.Status = models.TaskStatusDispatched
	task.Deadline = &deadline
	if err := d.TaskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task dispatched: %w", err)
	}

	payload := dispatchPayload{
		TaskID:       task.ID,
		Capability:   task.CapabilityRequired,
		InputPayload: task.InputPayload,
		CallbackURL:  fmt.Sprintf("/v1/tasks/%s/callback", task.ID),
		Deadline:     deadline,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dispatch payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, worker.EndpointURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dispatch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		d.Logger.Warn("worker dispatch failed, triggering fallback",
			"task_id", task.ID, "worker_id", worker.ID, "error", err)
		return d.dispatchWithFallback(ctx, task, worker.ID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.Logger.Warn("worker returned non-200, triggering fallback",
			"task_id", task.ID, "worker_id", worker.ID, "status", resp.StatusCode)
		return d.dispatchWithFallback(ctx, task, worker.ID)
	}

	task.Status = models.TaskStatusInProgress
	if err := d.TaskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task in_progress: %w", err)
	}

	go d.monitorDeadline(task.ID, worker.ID, deadline)

	return nil
}

// monitorDeadline sleeps until the deadline, then checks whether the task is
// still in_progress. If so it marks the task failed and triggers fallback.
func (d *Dispatcher) monitorDeadline(taskID, workerAgentID uuid.UUID, deadline time.Time) {
	remaining := time.Until(deadline)
	if remaining > 0 {
		time.Sleep(remaining)
	}

	ctx := context.Background()
	task, err := d.TaskRepo.GetByID(ctx, taskID)
	if err != nil {
		d.Logger.Error("deadline monitor: fetch task failed", "task_id", taskID, "error", err)
		return
	}
	if task.Status != models.TaskStatusInProgress {
		return
	}

	d.Logger.Warn("task deadline exceeded, marking failed", "task_id", taskID)
	task.Status = models.TaskStatusFailed
	task.OutputStatus = models.TaskOutputStatusError
	if err := d.TaskRepo.Update(ctx, task); err != nil {
		d.Logger.Error("deadline monitor: update task failed", "task_id", taskID, "error", err)
		return
	}

	if err := d.dispatchWithFallback(ctx, task, workerAgentID); err != nil {
		d.Logger.Error("deadline monitor: fallback dispatch failed", "task_id", taskID, "error", err)
	}
}

// dispatchWithFallback increments retry count and either dispatches to a
// fallback worker or refunds if retries are exhausted.
func (d *Dispatcher) dispatchWithFallback(ctx context.Context, task *models.Task, failedAgentID uuid.UUID) error {
	task.RetryCount++

	if task.RetryCount > maxRetries {
		return d.refundAndFail(ctx, task)
	}

	task.Status = models.TaskStatusMatching
	if err := d.TaskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task matching on retry: %w", err)
	}

	fallbacks, err := d.Matcher.FindFallbacks(ctx, task, failedAgentID)
	if err != nil {
		return fmt.Errorf("find fallbacks: %w", err)
	}
	if len(fallbacks) == 0 {
		return d.refundAndFail(ctx, task)
	}

	return d.dispatchToWorker(ctx, task, fallbacks[0])
}

// refundAndFail marks the task as permanently failed and refunds the full budget.
func (d *Dispatcher) refundAndFail(ctx context.Context, task *models.Task) error {
	task.Status = models.TaskStatusFailed
	task.OutputStatus = models.TaskOutputStatusError
	if err := d.TaskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task failed: %w", err)
	}

	requesterAccountID, err := d.resolveAccountID(ctx, task.RequesterAgentID)
	if err != nil {
		return fmt.Errorf("resolve requester account: %w", err)
	}

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := d.Escrow.RefundFailed(ctx, tx, task.ID, requesterAccountID, task.Budget); err != nil {
		return fmt.Errorf("refund failed: %w", err)
	}
	return tx.Commit(ctx)
}

// resolveAccountID looks up the account that owns the given agent.
func (d *Dispatcher) resolveAccountID(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error) {
	agent, err := d.AgentRepo.GetByID(ctx, agentID)
	if err != nil {
		return uuid.Nil, err
	}
	return agent.AccountID, nil
}

// failNoWorker marks the task failed and refunds when no workers are available at all.
func (d *Dispatcher) failNoWorker(ctx context.Context, task *models.Task) error {
	d.Logger.Warn("no workers available", "task_id", task.ID)
	return d.refundAndFail(ctx, task)
}

// BeginTx is a convenience for callers that need the pool. Exported so handlers
// can open a transaction that spans escrow + dispatch.
func (d *Dispatcher) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return d.Pool.Begin(ctx)
}
