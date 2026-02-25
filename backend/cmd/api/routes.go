package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inaiurai/backend/internal/handlers"
	"github.com/inaiurai/backend/internal/middleware"
	"github.com/inaiurai/backend/internal/repository"
	"github.com/inaiurai/backend/internal/services"
)

// RegisterV1Routes adds the /v1/ task API endpoints to the given mux.
// Middleware chain: APIKeyAuth -> (BudgetCheck on POST /v1/tasks only) -> handler.
func RegisterV1Routes(
	mux *http.ServeMux,
	pool *pgxpool.Pool,
	apiKeyRepo *repository.APIKeyRepo,
	agentRepo *repository.AgentRepo,
	taskRepo *repository.TaskRepo,
	accountRepo *repository.AccountRepo,
	creditRepo *repository.CreditRepo,
	validator *services.Validator,
	logger *slog.Logger,
) {
	matcher := services.NewMatcher(agentRepo)
	escrow := services.NewEscrowService(accountRepo, creditRepo)
	dispatcher := services.NewDispatcher(pool, matcher, validator, escrow, taskRepo, agentRepo, logger)

	th := &handlers.TaskHandler{
		Pool:       pool,
		TaskRepo:   taskRepo,
		AgentRepo:  agentRepo,
		Escrow:     escrow,
		Dispatcher: dispatcher,
		Validator:  validator,
		Logger:     logger,
	}

	auth := middleware.APIKeyAuth(apiKeyRepo, agentRepo)
	budgetAuth := middleware.BudgetCheck(pool)

	// POST /v1/tasks — Auth -> Budget -> CreateTask
	mux.Handle("POST /v1/tasks", auth(budgetAuth(http.HandlerFunc(th.CreateTask))))

	// POST /v1/tasks/{id}/result — Auth -> SubmitResult (worker callback)
	mux.Handle("POST /v1/tasks/{id}/result", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		th.SubmitResult(w, r)
	})))

	// GET /v1/tasks/{id} — Auth -> GetTask
	mux.Handle("GET /v1/tasks/{id}", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		th.GetTask(w, r)
	})))

	// GET /v1/tasks — Auth -> List tasks (convenience, returns requester's tasks)
	mux.Handle("GET /v1/tasks", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		listTasks(w, r, taskRepo, logger)
	})))
}

func listTasks(w http.ResponseWriter, r *http.Request, taskRepo *repository.TaskRepo, logger *slog.Logger) {
	acc := middleware.AccountFromCtx(r.Context())
	if acc == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	agent := middleware.AgentFromCtx(r.Context())
	if agent == nil {
		http.Error(w, `{"error":"no agent found for account"}`, http.StatusBadRequest)
		return
	}
	tasks, err := taskRepo.ListByRequesterAgentID(r.Context(), agent.ID)
	if err != nil {
		logger.Error("list tasks", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

