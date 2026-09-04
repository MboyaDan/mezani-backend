package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mezzani_backend/internal/ai"
	"mezzani_backend/internal/cache"
	"mezzani_backend/internal/config"
	"mezzani_backend/internal/database"
	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/notifications"
	"mezzani_backend/internal/router"
	"mezzani_backend/internal/service"
	"mezzani_backend/internal/workers"
)

func runMigrations(databaseURL string, logger *slog.Logger) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration failed: %w", err)
	}

	logger.Info("migrations applied successfully")
	return nil
}

func main() {
	ctx := context.Background()

	// ================= CONFIG =================
	cfg := config.LoadConfig()

	logger.Info("config check",
		"db_url_len", len(cfg.DatabaseURL),
		"db_url_prefix", cfg.DatabaseURL[:min(20, len(cfg.DatabaseURL))],
		"db_url_has_quotes", strings.Contains(cfg.DatabaseURL, `"`) || strings.Contains(cfg.DatabaseURL, "'"),
	)

	// ================= LOGGING =================
	// Constructed first so every service and subsystem gets the same logger.
	// JSON format is required for production log aggregators (Datadog, Loki, etc).
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting Mezzani Backend")

	// ================= MIGRATIONS =================
	if err := runMigrations(cfg.DatabaseURL, logger); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	// ================= DATABASE =================
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid database configuration", "error", err)
		os.Exit(1)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	dbpool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := dbpool.Ping(ctx); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	queries := db.New(dbpool)

	// ================= REDIS =================
	redisClient := database.NewRedisClient(cfg.RedisURL)
	eventBus := notifications.NewEventBus(redisClient)
	aiClient := ai.NewClient(cfg.GroqAPIKey, cfg.GroqModel, logger)

	// ================= CACHE =================
	appCache := cache.NewCache(redisClient)

	// ================= WEBSOCKET HUB =================
	hub := notifications.NewHub()
	go hub.Run()

	// ================= EXTERNAL SERVICES =================
	whatsapp := notifications.NewWhatsAppSender(
		cfg.WhatsappToken,
		cfg.WhatsappPhoneID,
	)

	emailSender := notifications.NewEmailSender(cfg.ResendAPIKey, cfg.ResendFromEmail)

	alertService := notifications.NewAlertService(cfg.TelegramBotToken, cfg.TelegramChatID)

	// ================= WORKERS =================
	// ================= WORKERS =================
	// ================= WORKERS =================
	go notifications.StartWhatsAppWorker(eventBus, whatsapp)
	go notifications.StartKitchenWorker(eventBus, hub)

	paymentsWorkerReady := make(chan struct{})
	go notifications.StartPaymentsWorker(eventBus, hub, paymentsWorkerReady)
	<-paymentsWorkerReady // block until the subscription is confirmed live — see StartPaymentsWorker's doc comment

	go workers.StartSessionExpiryWorker(queries)
	// ================= CORE SERVICES =================
	activityService := service.NewActivityService(queries)
	inventoryService := service.NewInventoryService(queries)

	// ================= BUSINESS SERVICES =================
	authService := service.NewAuthService(queries, []byte(cfg.JWTSecret), redisClient, logger)
	superAdminService := service.NewSuperAdminService(queries, []byte(cfg.JWTSecret), redisClient, logger)

	paystackClient := service.NewPaystackClient(cfg.PaystackSecretKey)
	subscriptionService := service.NewSubscriptionService(queries, paystackClient, cfg.FrontendURL, logger)
	menuService := service.NewMenuService(queries, appCache)
	branchService := service.NewBranchService(queries)
	tableService := service.NewTableService(queries)
	tableSessionService := service.NewTableSessionService(queries)
	customerService := service.NewCustomerSessionService(queries)
	cartService := service.NewSharedCartService(queries)
	analyticsService := service.NewAnalyticsService(queries)
	passwordResetService := service.NewPasswordResetService(queries, redisClient, emailSender, cfg.FrontendURL)
	staffService := service.NewStaffService(queries)
	aiService := service.NewAIService(queries, redisClient, aiClient, logger)

	orderService := service.NewOrderService(
		queries,
		eventBus,
		activityService,
		inventoryService,
	)
	billingService := service.NewBillingService(
		queries,
		eventBus,
		activityService,
	)

	paymentService := service.NewPaymentService(
		queries,
		billingService,
		eventBus,
		activityService,
	)

	// ================= HANDLERS =================
	authHandler := handler.NewAuthHandler(authService)
	superAdminHandler := handler.NewSuperAdminHandler(superAdminService, logger)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService, paystackClient, logger)
	menuHandler := handler.NewMenuHandler(menuService, tableSessionService)
	branchHandler := handler.NewBranchHandler(branchService)
	tableHandler := handler.NewTableHandler(tableService)
	tableSessionHandler := handler.NewTableSessionHandler(tableSessionService)
	customerHandler := handler.NewCustomerSessionHandler(customerService)
	cartHandler := handler.NewSharedCartHandler(cartService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)
	orderHandler := handler.NewOrderHandler(orderService)
	billingHandler := handler.NewBillingHandler(billingService)
	paymentHandler := handler.NewPaymentHandler(paymentService)

	staffHandler := handler.NewStaffHandler(authService, staffService,
		func(ctx context.Context, tenantID uuid.UUID) (string, time.Time, error) {
			row, err := queries.GetTenantSubscriptionStatus(ctx, tenantID)
			if err != nil {
				return "", time.Time{}, err
			}
			return row.SubscriptionStatus, row.SubscriptionExpiresAt, nil
		},
	)

	inventoryHandler := handler.NewInventoryHandler(inventoryService)
	wsHandler := handler.NewWSHandler(hub)
	passwordResetHandler := handler.NewPasswordResetHandler(passwordResetService)
	aiHandler := handler.NewAIHandler(aiService)

	// ================= ROUTER =================
	r := router.SetupRouter(
		[]byte(cfg.JWTSecret),
		cfg.AllowedOrigins,
		branchService,
		alertService,
		authHandler,
		tableSessionHandler,
		customerHandler,
		cartHandler,
		orderHandler,
		wsHandler,
		menuHandler,
		billingHandler,
		paymentHandler,
		analyticsHandler,
		staffHandler,
		tableHandler,
		branchHandler,
		inventoryHandler,
		passwordResetHandler,
		aiHandler,
		superAdminHandler,
		subscriptionHandler,
		func(ctx context.Context, tenantID uuid.UUID) (time.Time, error) {
			row, err := queries.GetTenantSubscriptionStatus(ctx, tenantID)
			if err != nil {
				return time.Time{}, err
			}
			return row.SubscriptionExpiresAt, nil
		},
	)

	// ================= SERVER =================
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start server in a goroutine so it doesn't block signal handling below.
	go func() {
		logger.Info("server listening", "port", cfg.Port)

		// Fire the Telegram alert only after the server has actually started.
		go alertService.Info("Mezzani Started", fmt.Sprintf("Server running on port %s", cfg.Port))

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// ================= GRACEFUL SHUTDOWN =================
	// Block until we receive SIGINT or SIGTERM (container stop, deploy, Ctrl+C).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received, draining connections...")

	// Give in-flight requests up to 10 seconds to complete before forcing exit.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped cleanly")
}
