package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/inaiurai/backend/internal/models"
	"github.com/inaiurai/backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type stubAPIKeyRepo struct {
	result *repository.APIKeyWithAccount
	err    error
}

func (s *stubAPIKeyRepo) FindByKeyHash(_ context.Context, _ string) (*repository.APIKeyWithAccount, error) {
	return s.result, s.err
}

type stubAgentLookup struct {
	agents []*models.Agent
}

func (s *stubAgentLookup) ListByAccountID(_ context.Context, _ uuid.UUID) ([]*models.Agent, error) {
	return s.agents, nil
}

// okHandler writes 200 and the account email (for assertions).
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	acc := AccountFromCtx(r.Context())
	if acc != nil {
		w.Write([]byte(acc.Email))
	}
	w.WriteHeader(http.StatusOK)
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	account := models.Account{
		ID:    uuid.New(),
		Email: "test@example.com",
	}
	agent := &models.Agent{ID: uuid.New(), AccountID: account.ID}

	repo := &stubAPIKeyRepo{
		result: &repository.APIKeyWithAccount{
			APIKey:  models.APIKey{ID: uuid.New(), AccountID: account.ID, IsActive: true},
			Account: account,
		},
	}
	lookup := &stubAgentLookup{agents: []*models.Agent{agent}}

	mw := APIKeyAuth(repo, lookup)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-test-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != account.Email {
		t.Errorf("expected account email %q in body, got %q", account.Email, body)
	}
}

func TestAPIKeyAuth_MissingHeader(t *testing.T) {
	repo := &stubAPIKeyRepo{}
	lookup := &stubAgentLookup{}
	mw := APIKeyAuth(repo, lookup)(okHandler)

	cases := []struct {
		name   string
		header string
	}{
		{"no header at all", ""},
		{"empty bearer", "Bearer "},
		{"wrong scheme", "Basic abc123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestAPIKeyAuth_InvalidOrRevokedKey(t *testing.T) {
	repo := &stubAPIKeyRepo{err: errors.New("not found")}
	lookup := &stubAgentLookup{}
	mw := APIKeyAuth(repo, lookup)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer revoked-or-invalid-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
