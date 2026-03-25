package main

import (
	"context"
	"fmt"
	"log"

	"mezzani_backend/internal/cache"
	"mezzani_backend/internal/config"
	"mezzani_backend/internal/database"
	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/notifications"
	"mezzani_backend/internal/router"
	"mezzani_backend/internal/service"
	"mezzani_backend/internal/workers"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgx/v5/pgxpool"
)

func runMigrations(databaseURL string) {
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		log.Fatal("Migration init error:", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal("Migration failed:", err)
	}

	log.Println("Migrations applied successfully")
}

func main() {
	ctx := context.Background()

	// ================= CONFIG =================
	cfg := config.LoadConfig()

	// ================= MIGRATIONS =================
	runMigrations(cfg.DatabaseURL)

	// ================= DATABASE =================
	dbpool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	defer dbpool.Close()

	queries := db.New(dbpool)

	// ================= REDIS =================
	redisClient := database.NewRedisClient(cfg.RedisURL)
	eventBus := notifications.NewEventBus(redisClient)

	//cache
	appCache := cache.NewCache(redisClient)

	// ================= LOGGING =================
	log.Println("Starting Mezzani Backend...")

	// ================= WEBSOCKET HUB =================
	hub := notifications.NewHub()
	go hub.Run()

	// ================= EXTERNAL SERVICES =================
	whatsapp := notifications.NewWhatsAppSender(
		cfg.WhatsappToken,
		cfg.WhatsappPhoneID,
	)

	// ================= CORE SERVICES (SINGLE INSTANCES) =================
	activityService := service.NewActivityService(queries)
	inventoryService := service.NewInventoryService(queries)

	// ================= WORKERS =================
	go notifications.StartWhatsAppWorker(eventBus, whatsapp)
	go notifications.StartKitchenWorker(eventBus, hub)
	go workers.StartSessionExpiryWorker(queries)
	//go workers.StartInventoryWorker(queries) // Optional: Start inventory worker for periodic stock checks

	// ================= BUSINESS SERVICES =================
	tableSessionService := service.NewTableSessionService(queries)
	customerService := service.NewCustomerSessionService(queries)
	cartService := service.NewSharedCartService(queries)
	analyticsService := service.NewAnalyticsService(queries)

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

	authService := service.NewAuthService(queries, []byte(cfg.JWTSecret), redisClient)
	menuService := service.NewMenuService(queries, appCache)

	branchService := service.NewBranchService(queries)
	tableService := service.NewTableService(queries)

	// ================= EMAIL =================
	emailSender := notifications.NewEmailSender(cfg.ResendAPIKey, cfg.ResendFromEmail)

	// ================= PASSWORD RESET =================
	passwordResetService := service.NewPasswordResetService(queries, redisClient, emailSender, cfg.FrontendURL)

	//alert service (for logging panics and critical errors to Telegram)

	alertService := notifications.NewAlertService(cfg.TelegramBotToken, cfg.TelegramChatID)

	go alertService.Info("Mezzani Started", fmt.Sprintf("Server running on port %s", cfg.Port))

	// ================= HANDLERS =================
	tableSessionHandler := handler.NewTableSessionHandler(tableSessionService)
	customerHandler := handler.NewCustomerSessionHandler(customerService)
	cartHandler := handler.NewSharedCartHandler(cartService)
	orderHandler := handler.NewOrderHandler(orderService)
	wsHandler := handler.NewWSHandler(hub)
	authHandler := handler.NewAuthHandler(authService)
	billingHandler := handler.NewBillingHandler(billingService)
	menuHandler := handler.NewMenuHandler(menuService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)
	staffHandler := handler.NewStaffHandler(authService)
	branchHandler := handler.NewBranchHandler(branchService)
	tableHandler := handler.NewTableHandler(tableService)
	inventoryHandler := handler.NewInventoryHandler(inventoryService)
	passwordResetHandler := handler.NewPasswordResetHandler(passwordResetService)

	// ================= ROUTER =================
	r := router.SetupRouter(
		dbpool,
		[]byte(cfg.JWTSecret),
		cfg.AllowedOrigins,
		branchService,
		alertService, // Pass alert service to router for panic recovery

		// Handlers
		authHandler,
		tableSessionHandler,
		customerHandler,
		cartHandler,
		orderHandler,
		wsHandler,
		menuHandler,
		billingHandler,
		analyticsHandler,
		staffHandler,
		tableHandler,
		branchHandler,
		inventoryHandler,
		passwordResetHandler,
	)

	// ================= SERVER =================
	log.Println("Server starting on port:", cfg.Port)

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Server failed:", err)
	}
}
