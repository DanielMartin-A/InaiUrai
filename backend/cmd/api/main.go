package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/rs/cors"

	"github.com/inaiurai/backend/internal/auth"
	"github.com/inaiurai/backend/internal/dashboard"
	"github.com/inaiurai/backend/internal/execution"
	"github.com/inaiurai/backend/internal/handlers"
	"github.com/inaiurai/backend/internal/jobs"
	"github.com/inaiurai/backend/internal/ledger"
	"github.com/inaiurai/backend/internal/registry"
	"github.com/inaiurai/backend/internal/repository"
	"github.com/inaiurai/backend/internal/router"
	"github.com/inaiurai/backend/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://inaiurai_dev:devpassword@localhost:5432/inaiurai?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("Unable to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("Cannot reach PostgreSQL (connection refused or invalid). Ensure Postgres is running, e.g. make dev-up or docker-compose up -d", "error", err)
		os.Exit(1)
	}
	slog.Info("Connected to PostgreSQL database successfully!")

	// River migrations
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		slog.Error("Failed to create River migrator", "error", err)
		os.Exit(1)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		slog.Error("River migrate up failed. If the error is 'connection refused', start PostgreSQL first (e.g. make dev-up)", "error", err)
		os.Exit(1)
	}
	slog.Info("River migrations applied")

	// Ledger
	ledgerRepo := ledger.NewRepository(pool)
	ledgerSvc := ledger.NewService(ledgerRepo)

	// Jobs: insert func is set after River client is created (breaks init cycle)
	var insertMu sync.Mutex
	var insertFn jobs.InsertExecuteAgentTxFunc
	insertExecuteAgent := func(ctx context.Context, tx pgx.Tx, args execution.ExecuteAgentJobArgs) error {
		insertMu.Lock()
		fn := insertFn
		insertMu.Unlock()
		if fn == nil {
			panic("river insert not wired")
		}
		return fn(ctx, tx, args)
	}

	jobsRepo := jobs.NewRepository(pool)
	jobsSvc := jobs.NewService(jobsRepo, ledgerSvc, insertExecuteAgent)

	// Execution worker (implements JobService via jobsSvc)
	workers := river.NewWorkers()
	river.AddWorker(workers, execution.NewExecuteAgentWorker(jobsSvc))

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: workers,
	})
	if err != nil {
		slog.Error("Failed to create River client", "error", err)
		os.Exit(1)
	}

	insertMu.Lock()
	insertFn = func(ctx context.Context, tx pgx.Tx, args execution.ExecuteAgentJobArgs) error {
		_, err := riverClient.InsertTx(ctx, tx, args, nil)
		return err
	}
	insertMu.Unlock()

	// Auth & Registry
	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo)
	authHandler := auth.NewHandler(authSvc, logger)

	registryRepo := registry.NewRepository(pool)
	registrySvc := registry.NewService(registryRepo)
	registryHandler := registry.NewHandler(registrySvc, authSvc, logger)

	jobsHandler := jobs.NewHandler(jobs.Service(jobsSvc), authSvc, logger)

	// Cursorrules-schema repositories (v1 task API)
	apiKeyRepo := repository.NewAPIKeyRepo(pool)
	agentRepo := repository.NewAgentRepo(pool)
	taskRepo := repository.NewTaskRepo(pool)
	accountRepo := repository.NewAccountRepo(pool)
	creditRepo := repository.NewCreditRepo(pool)

	dashHandler := dashboard.NewHandler(authSvc, accountRepo, creditRepo, apiKeyRepo, taskRepo, agentRepo, logger)

	apiV1Router := router.New(authHandler, registryHandler, jobsHandler, dashHandler)

	schemaDir := os.Getenv("SCHEMA_DIR")
	if schemaDir == "" {
		schemaDir = "schemas"
	}
	validator, err := services.NewValidator(ctx, schemaDir)
	if err != nil {
		slog.Warn("Schema validator init failed (v1 task routes disabled)", "error", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", apiV1Router)

	if validator != nil {
		RegisterV1Routes(mux, pool, apiKeyRepo, agentRepo, taskRepo, accountRepo, creditRepo, validator, logger)
	}
	mux.HandleFunc("GET /v1/capabilities", handlers.ListCapabilities)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://stellar-gentleness-production-16de.up.railway.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}).Handler(mux)

	// Start River client (processes jobs)
	riverCtx, stopRiver := context.WithCancel(ctx)
	defer stopRiver()
	go func() {
		if err := riverClient.Start(riverCtx); err != nil && riverCtx.Err() == nil {
			slog.Error("River client stopped", "error", err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Fallback for local development
	}
	serverAddr := "0.0.0.0:" + port

	slog.Info("Starting HTTP server", "addr", serverAddr)
	if err := http.ListenAndServe(serverAddr, corsHandler); err != nil {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
