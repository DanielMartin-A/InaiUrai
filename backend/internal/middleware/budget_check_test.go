package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/models"
)

// injectAccount wraps a handler to pre-set the account in context,
// simulating what APIKeyAuth would do upstream.
func injectAccount(acc *models.Account, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxAccountKey, acc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func intP(n int) *int { return &n }

// budget200 is a handler that writes 200 OK; it proves the middleware let the
// request through.
var budget200 = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// ---------------------------------------------------------------------------
// 1. Request within limits -> 200 OK
// ---------------------------------------------------------------------------

func TestBudgetCheck_WithinLimits(t *testing.T) {
	original := dailySpendFn
	dailySpendFn = func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID) (int, error) {
		return 0, nil
	}
	defer func() { dailySpendFn = original }()

	acc := &models.Account{
		ID:               uuid.New(),
		GlobalMaxPerTask: intP(50),
		GlobalMaxPerDay:  intP(200),
	}

	handler := injectAccount(acc, BudgetCheck(nil)(budget200))

	body := `{"budget":30,"capability_required":"research"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 2. Budget > max_per_task -> 403 (TASK_BUDGET_EXCEEDED)
// ---------------------------------------------------------------------------

func TestBudgetCheck_ExceedsPerTask(t *testing.T) {
	acc := &models.Account{
		ID:               uuid.New(),
		GlobalMaxPerTask: intP(20),
	}

	handler := injectAccount(acc, BudgetCheck(nil)(budget200))

	body := `{"budget":50,"capability_required":"research"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "exceeds per-task limit") {
		t.Errorf("expected per-task error message, got: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 3. Daily spend + budget > max_per_day -> 403 (DAILY_BUDGET_EXCEEDED)
// ---------------------------------------------------------------------------

func TestBudgetCheck_ExceedsDailyLimit(t *testing.T) {
	original := dailySpendFn
	dailySpendFn = func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID) (int, error) {
		return 180, nil // already spent 180 today
	}
	defer func() { dailySpendFn = original }()

	acc := &models.Account{
		ID:               uuid.New(),
		GlobalMaxPerTask: intP(100),
		GlobalMaxPerDay:  intP(200),
	}

	handler := injectAccount(acc, BudgetCheck(nil)(budget200))

	// 180 spent + 30 requested = 210 > 200 limit
	body := `{"budget":30,"capability_required":"research"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "exceeds daily limit") {
		t.Errorf("expected daily limit error message, got: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 4. Unknown capability -> 403
// ---------------------------------------------------------------------------

func TestBudgetCheck_UnknownCapability(t *testing.T) {
	acc := &models.Account{
		ID: uuid.New(),
	}

	handler := injectAccount(acc, BudgetCheck(nil)(budget200))

	body := `{"budget":10,"capability_required":"teleportation"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not allowed") {
		t.Errorf("expected capability-not-allowed error, got: %s", rec.Body.String())
	}
}
