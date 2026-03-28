package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/handlers"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/middleware"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
	wsHub "github.com/DanielMartin-A/InaiUrai/backend/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		slog.Error("database ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	orgRepo := repository.NewOrgRepo(db)
	memberRepo := repository.NewMemberRepo(db)
	engagementRepo := repository.NewEngagementRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	costRepo := repository.NewCostRepo(db)
	contextRepo := repository.NewContextRepo(db)
	billingRepo := repository.NewBillingRepo(db)

	engineURL := os.Getenv("ENGINE_URL")
	if engineURL == "" {
		engineURL = "http://engine:8000"
	}
	internalKey := os.Getenv("INTERNAL_API_KEY")

	engineClient := services.NewEngineClient(engineURL, internalKey)
	engagementSvc := services.NewEngagementService(engagementRepo, taskRepo, orgRepo, engineClient, contextRepo)
	taskManager := services.NewTaskManager(taskRepo, orgRepo, memberRepo, engineClient, engagementSvc)
	billingSvc := services.NewBillingService(billingRepo, orgRepo)
	telegramSvc := services.NewTelegramService(os.Getenv("TELEGRAM_BOT_TOKEN"), memberRepo, orgRepo, taskManager)
	slackSvc := services.NewSlackService(os.Getenv("SLACK_SIGNING_SECRET"), os.Getenv("SLACK_BOT_TOKEN"), memberRepo, orgRepo, taskManager)
	whatsappSvc := services.NewWhatsAppService(
		os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		os.Getenv("WHATSAPP_APP_SECRET"), os.Getenv("WHATSAPP_VERIFY_TOKEN"),
		memberRepo, orgRepo, taskManager)
	emailSvc := services.NewEmailService(os.Getenv("EMAIL_DOMAIN"), memberRepo, orgRepo, taskManager)
	smsSvc := services.NewSMSService(os.Getenv("TWILIO_ACCOUNT_SID"), os.Getenv("TWILIO_AUTH_TOKEN"), os.Getenv("TWILIO_PHONE_NUMBER"))
	heartbeatSvc := services.NewHeartbeatService(engagementRepo, taskManager)

	telegramHandler := handlers.NewTelegramHandler(telegramSvc)
	billingHandler := handlers.NewBillingHandler(billingSvc)
	slackHandler := handlers.NewSlackHandler(slackSvc)
	whatsappHandler := handlers.NewWhatsAppHandler(whatsappSvc)
	emailHandler := handlers.NewEmailHandler(emailSvc)
	internalHandler := handlers.NewInternalHandler(contextRepo, costRepo, auditRepo, orgRepo)
	apiHandler := handlers.NewAPIv1Handler(orgRepo, memberRepo, engagementRepo, taskRepo, engineClient, engagementSvc, taskManager)

	hub := wsHub.NewHub()
	taskManager.SetBroadcaster(hub)
	wsHandler := handlers.NewWSHandler(hub, memberRepo, taskManager, orgRepo)

	deliveryRouter := services.NewDeliveryRouter(hub, telegramSvc, whatsappSvc, slackSvc, smsSvc, memberRepo)
	taskManager.SetDeliveryRouter(deliveryRouter)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","version":"5.0"}`)
	})

	botHash := middleware.WebhookHash(os.Getenv("TELEGRAM_BOT_TOKEN"))
	mux.HandleFunc("POST /webhook/telegram/"+botHash, telegramHandler.Handle)
	mux.HandleFunc("POST /webhook/stripe", billingHandler.Handle)
	mux.HandleFunc("POST /webhook/slack/events", slackHandler.HandleEvent)
	mux.HandleFunc("POST /webhook/slack/interact", slackHandler.HandleInteraction)
	mux.HandleFunc("GET /webhook/whatsapp", whatsappHandler.HandleVerify)
	mux.HandleFunc("POST /webhook/whatsapp", whatsappHandler.HandleMessage)
	mux.HandleFunc("POST /webhook/email/inbound", emailHandler.HandleInbound)

	mux.Handle("/ws/chat", wsHandler)

	internal := http.NewServeMux()
	internal.HandleFunc("POST /api/context/selective", internalHandler.SelectiveContext)
	internal.HandleFunc("GET /api/internal/daily-cost/{orgID}", internalHandler.DailyCost)
	internal.HandleFunc("POST /api/internal/record-cost", internalHandler.RecordCost)
	internal.HandleFunc("POST /api/internal/audit", internalHandler.StoreAudit)
	internal.HandleFunc("GET /api/internal/audit/{taskID}", internalHandler.GetAudit)
	internal.HandleFunc("POST /api/internal/create-token", internalHandler.CreateToken)
	mux.Handle("/api/internal/", middleware.RequireInternalKey(internalKey, internal))
	mux.Handle("/api/context/", middleware.RequireInternalKey(internalKey, internal))

	apiv1 := http.NewServeMux()
	apiv1.HandleFunc("POST /api/v1/engagements", apiHandler.CreateEngagement)
	apiv1.HandleFunc("GET /api/v1/engagements", apiHandler.ListEngagements)
	apiv1.HandleFunc("GET /api/v1/engagements/{id}", apiHandler.GetEngagement)
	apiv1.HandleFunc("GET /api/v1/engagements/{id}/tasks", apiHandler.ListTasks)
	apiv1.HandleFunc("GET /api/v1/engagements/{id}/trace", apiHandler.GetTrace)
	apiv1.HandleFunc("POST /api/v1/tasks", apiHandler.CreateTask)
	apiv1.HandleFunc("GET /api/v1/org", apiHandler.GetOrg)
	apiv1.HandleFunc("GET /api/v1/roles", apiHandler.ListRoles)
	mux.Handle("/api/v1/", middleware.RequireBearerToken(orgRepo, memberRepo, apiv1))

	handler := middleware.CORS(os.Getenv("ALLOWED_ORIGINS"),
		middleware.RateLimit(60,
			mux))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heartbeatSvc.Run(ctx)

	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}
	slog.Info("server stopped")
}
