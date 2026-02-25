package registry

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type Service interface {
	CreateAgent(ctx context.Context, accountID uuid.UUID, name, description string, capabilities []string, basePriceCents int32, webhookURL string, inputSchema, outputSchema json.RawMessage) (*AgentProfile, error)
	ListActiveAgents(ctx context.Context) ([]*AgentProfile, error)
}

type service struct {
	repo *Repository
}

func NewService(repo *Repository) *service {
	return &service{repo: repo}
}

var _ Service = (*service)(nil)

var slugSanitize = regexp.MustCompile(`[^a-z0-9-]+`)

// normalizeCapabilities lowercases each capability so matching is case-insensitive.
func normalizeCapabilities(capabilities []string) []string {
	out := make([]string, len(capabilities))
	for i, c := range capabilities {
		out[i] = strings.ToLower(strings.TrimSpace(c))
	}
	return out
}

func slugFromName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugSanitize.ReplaceAllString(s, "")
	if s == "" {
		s = "agent"
	}
	return s + "-" + uuid.New().String()[:8]
}

func (s *service) CreateAgent(ctx context.Context, accountID uuid.UUID, name, description string, capabilities []string, basePriceCents int32, webhookURL string, inputSchema, outputSchema json.RawMessage) (*AgentProfile, error) {
	slug := slugFromName(name)
	return s.repo.Create(ctx, CreateParams{
		AccountID:      accountID,
		Name:           name,
		Slug:           slug,
		Description:    description,
		Capabilities:   normalizeCapabilities(capabilities),
		BasePriceCents: basePriceCents,
		WebhookURL:     webhookURL,
		InputSchema:    inputSchema,
		OutputSchema:   outputSchema,
	})
}

func (s *service) ListActiveAgents(ctx context.Context) ([]*AgentProfile, error) {
	return s.repo.ListActive(ctx)
}
