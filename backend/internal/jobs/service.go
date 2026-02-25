package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/inaiurai/backend/internal/execution"
	"github.com/inaiurai/backend/internal/ledger"
)

type Job struct {
	ID                   uuid.UUID
	RequesterID          uuid.UUID
	Title                string
	Description          string
	Status               string
	BudgetCents          int64
	AgreedPriceCents     *int64
	RequiredCapabilities []string
	InputPayload         json.RawMessage
	AssignedAgentID      *uuid.UUID
	OutputPayload        json.RawMessage
}

type Service interface {
	CreateJob(ctx context.Context, requesterID uuid.UUID, title, desc string, capabilities []string, budget int64, input json.RawMessage) (*Job, error)
	AssignAgent(ctx context.Context, jobID uuid.UUID) error
	GetJob(ctx context.Context, jobID uuid.UUID) (*Job, error)
	ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*Job, error)
}

// InsertExecuteAgentTxFunc enqueues an ExecuteAgent job within the given transaction. Provided by main using river.Client.InsertTx.
type InsertExecuteAgentTxFunc func(ctx context.Context, tx pgx.Tx, args execution.ExecuteAgentJobArgs) error

type service struct {
	repo               *Repository
	ledger             ledger.Service
	insertExecuteAgent InsertExecuteAgentTxFunc
}

// NewService creates a jobs service. insertExecuteAgent is typically a closure over river.Client.InsertTx.
// Returns *service so it can be used as execution.JobService for the River worker.
func NewService(repo *Repository, ledger ledger.Service, insertExecuteAgent InsertExecuteAgentTxFunc) *service {
	return &service{repo: repo, ledger: ledger, insertExecuteAgent: insertExecuteAgent}
}

var _ Service = (*service)(nil)

func (s *service) CreateJob(ctx context.Context, requesterID uuid.UUID, title, desc string, capabilities []string, budget int64, input json.RawMessage) (*Job, error) {
	return s.repo.Create(ctx, requesterID, title, desc, normalizeCapabilities(capabilities), budget, input)
}

// normalizeCapabilities lowercases each capability so matching is case-insensitive.
func normalizeCapabilities(capabilities []string) []string {
	out := make([]string, len(capabilities))
	for i, c := range capabilities {
		out[i] = strings.ToLower(strings.TrimSpace(c))
	}
	return out
}

func (s *service) AssignAgent(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "OPEN" {
		return errors.New("job is not open for assignment")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	agent, err := s.repo.FindMatchingAgent(ctx, tx, job.RequiredCapabilities, job.BudgetCents)
	if err != nil {
		return err
	}
	agreed := int64(agent.BasePriceCents)
	if err := s.repo.UpdateAssigned(ctx, tx, jobID, agent.AgentID, agreed); err != nil {
		return err
	}
	if err := s.ledger.PlaceEscrowHold(ctx, tx, jobID, job.RequesterID, agreed); err != nil {
		if errors.Is(err, ledger.ErrInsufficientFunds) {
			return err
		}
		return err
	}
	if err := s.insertExecuteAgent(ctx, tx, execution.ExecuteAgentJobArgs{
		JobID:      jobID,
		AgentID:    agent.AgentID,
		WebhookURL: agent.WebhookURL,
		Payload:    job.InputPayload,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *service) GetJob(ctx context.Context, jobID uuid.UUID) (*Job, error) {
	return s.repo.GetByID(ctx, jobID)
}

func (s *service) ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*Job, error) {
	return s.repo.ListByRequester(ctx, requesterID)
}

// MarkJobCompleted implements execution.JobService. Updates job to SETTLED and releases escrow to the provider.
func (s *service) MarkJobCompleted(ctx context.Context, jobID uuid.UUID, outputPayload []byte) error {
	if err := s.repo.MarkJobCompleted(ctx, jobID, outputPayload); err != nil {
		return err
	}
	providerID, err := s.repo.GetProviderAccountID(ctx, jobID)
	if err != nil {
		return err
	}
	return s.ledger.ReleaseEscrow(ctx, jobID, providerID)
}

// MarkJobFailed implements execution.JobService. Updates job to FAILED and refunds escrow to the requester.
func (s *service) MarkJobFailed(ctx context.Context, jobID uuid.UUID, reason string) error {
	if err := s.repo.MarkJobFailed(ctx, jobID); err != nil {
		return err
	}
	return s.ledger.RefundEscrow(ctx, jobID)
}