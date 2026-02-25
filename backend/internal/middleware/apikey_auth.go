package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/inaiurai/backend/internal/models"
	"github.com/inaiurai/backend/internal/repository"
)

type contextKey string

const (
	ctxAccountKey contextKey = "account"
	ctxAgentKey   contextKey = "agent"
)

// APIKeyRepo is the interface used by API key auth middleware.
type APIKeyRepo interface {
	FindByKeyHash(ctx context.Context, keyHash string) (*repository.APIKeyWithAccount, error)
}

// AgentLookup is used to resolve the first agent for the authenticated account.
type AgentLookup interface {
	ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Agent, error)
}

// APIKeyAuth authenticates requests by hashing the Bearer token (SHA-256)
// and looking it up in api_keys. On success it sets account and the first
// matching agent into request context.
func APIKeyAuth(apiKeyRepo APIKeyRepo, agentLookup AgentLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractBearer(r)
			if raw == "" {
				http.Error(w, `{"error":"missing or malformed Authorization header"}`, http.StatusUnauthorized)
				return
			}

			hash := hashKey(raw)
			result, err := apiKeyRepo.FindByKeyHash(r.Context(), hash)
			if err != nil {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxAccountKey, &result.Account)

			agents, err := agentLookup.ListByAccountID(r.Context(), result.Account.ID)
			if err == nil && len(agents) > 0 {
				ctx = context.WithValue(ctx, ctxAgentKey, agents[0])
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccountFromCtx returns the authenticated account or nil.
func AccountFromCtx(ctx context.Context) *models.Account {
	acc, _ := ctx.Value(ctxAccountKey).(*models.Account)
	return acc
}

// WithAccount returns a context carrying the given account.
func WithAccount(ctx context.Context, acc *models.Account) context.Context {
	return context.WithValue(ctx, ctxAccountKey, acc)
}

// AgentFromCtx returns the first agent of the authenticated account, or nil.
func AgentFromCtx(ctx context.Context) *models.Agent {
	ag, _ := ctx.Value(ctxAgentKey).(*models.Agent)
	return ag
}

// WithAgent returns a context carrying the given agent.
func WithAgent(ctx context.Context, ag *models.Agent) context.Context {
	return context.WithValue(ctx, ctxAgentKey, ag)
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
