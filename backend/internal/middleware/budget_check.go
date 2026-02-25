package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ctxBudgetKey contextKey = "parsed_budget"

// AllowedCapabilities is the set of capabilities the platform supports.
// BudgetCheck rejects requests with unknown capabilities early.
var AllowedCapabilities = map[string]bool{
	"research":        true,
	"summarize":       true,
	"data_extraction": true,
}

// parsedBudget is stored in context so the handler can read the budget
// without re-parsing the body.
type parsedBudget struct {
	Budget             int    `json:"budget"`
	CapabilityRequired string `json:"capability_required"`
}

// BudgetFromCtx returns the budget parsed by BudgetCheck, or 0 if not set.
func BudgetFromCtx(ctx context.Context) int {
	if b, ok := ctx.Value(ctxBudgetKey).(*parsedBudget); ok {
		return b.Budget
	}
	return 0
}

// BudgetCheck validates per-task and daily budget limits from the
// account set by APIKeyAuth. Reads the body to extract "budget",
// then replaces r.Body so downstream handlers can re-read it.
func BudgetCheck(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acc := AccountFromCtx(r.Context())
			if acc == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			// Restore body for the handler.
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var peek parsedBudget
			if err := json.Unmarshal(bodyBytes, &peek); err != nil {
				http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
				return
			}
			if peek.Budget <= 0 {
				http.Error(w, `{"error":"budget must be > 0"}`, http.StatusBadRequest)
				return
			}

			if peek.CapabilityRequired != "" && !AllowedCapabilities[peek.CapabilityRequired] {
				http.Error(w, fmt.Sprintf(`{"error":"capability %q is not allowed"}`, peek.CapabilityRequired), http.StatusForbidden)
				return
			}

			if acc.GlobalMaxPerTask != nil && peek.Budget > *acc.GlobalMaxPerTask {
				http.Error(w, fmt.Sprintf(`{"error":"budget %d exceeds per-task limit %d"}`, peek.Budget, *acc.GlobalMaxPerTask), http.StatusForbidden)
				return
			}

			if acc.GlobalMaxPerDay != nil {
				spent, err := dailySpendFn(r.Context(), pool, acc.ID)
				if err != nil {
					http.Error(w, `{"error":"failed to check daily spend"}`, http.StatusInternalServerError)
					return
				}
				if spent+peek.Budget > *acc.GlobalMaxPerDay {
					http.Error(w, fmt.Sprintf(`{"error":"daily spend %d + budget %d exceeds daily limit %d"}`, spent, peek.Budget, *acc.GlobalMaxPerDay), http.StatusForbidden)
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxBudgetKey, &peek)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// dailySpendFn is the function used to compute today's spend.
// Tests can replace this to avoid hitting a real database.
var dailySpendFn = defaultDailySpend

// defaultDailySpend sums escrow_lock amounts for the account today (UTC).
func defaultDailySpend(ctx context.Context, pool *pgxpool.Pool, accountID uuid.UUID) (int, error) {
	var total int
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM credit_ledger
		WHERE account_id = $1 AND entry_type = 'escrow_lock'
		  AND created_at >= CURRENT_DATE
	`, accountID).Scan(&total)
	return total, err
}
