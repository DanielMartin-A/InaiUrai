package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type ExecuteAgentJobArgs struct {
	JobID      uuid.UUID       `json:"job_id"`
	AgentID    uuid.UUID       `json:"agent_id"`
	WebhookURL string          `json:"webhook_url"`
	Payload    json.RawMessage `json:"payload"`
}

func (ExecuteAgentJobArgs) Kind() string { return "execute_agent" }

// JobService defines the contract the worker needs to report success/failure
type JobService interface {
	MarkJobCompleted(ctx context.Context, jobID uuid.UUID, outputPayload []byte) error
	MarkJobFailed(ctx context.Context, jobID uuid.UUID, reason string) error
}

type ExecuteAgentWorker struct {
	river.WorkerDefaults[ExecuteAgentJobArgs]
	jobService JobService
	httpClient *http.Client
}

func NewExecuteAgentWorker(js JobService) *ExecuteAgentWorker {
	return &ExecuteAgentWorker{
		jobService: js,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (w *ExecuteAgentWorker) Work(ctx context.Context, job *river.Job[ExecuteAgentJobArgs]) error {
	args := job.Args

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, args.WebhookURL, bytes.NewReader(args.Payload))
	if err != nil {
		return w.failJob(ctx, args.JobID, fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network error calling agent webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return w.failJob(ctx, args.JobID, fmt.Sprintf("agent returned non-200 status: %d", resp.StatusCode))
	}

	var outputPayload json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&outputPayload); err != nil {
		return w.failJob(ctx, args.JobID, "agent returned invalid JSON")
	}

	err = w.jobService.MarkJobCompleted(ctx, args.JobID, outputPayload)
	if err != nil {
		return fmt.Errorf("failed to mark job completed: %w", err)
	}
	return nil
}

func (w *ExecuteAgentWorker) failJob(ctx context.Context, jobID uuid.UUID, reason string) error {
	markErr := w.jobService.MarkJobFailed(ctx, jobID, reason)
	if markErr != nil {
		return fmt.Errorf("agent failed (%s) AND failed to mark job as failed: %w", reason, markErr)
	}
	return nil
}
