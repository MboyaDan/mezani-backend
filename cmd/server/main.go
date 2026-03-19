package main

import (
	"context"
	"log"
	"os"

	"mezzani_backend/internal/database"
	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/notifications"
	"mezzani_backend/internal/router"
	"mezzani_backend/internal/service"
	"mezzani_backend/internal/workers"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// ================= CONFIG =================
	dbURL := getEnv("DATABASE_URL", "postgres://...")
	whatsAppToken := getEnv("WHATSAPP_TOKEN", "")
	phoneID := getEnv("WHATSAPP_PHONE_ID", "")
	jwtSecret := getEnv("JWT_SECRET", "fallback-secret-key")

	// ================= DATABASE =================
	dbpool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	defer dbpool.Close()

	queries := db.New(dbpool)

	// ================= REDIS =================
	redisClient := database.NewRedisClient()
	eventBus := notifications.NewEventBus(redisClient)

	// ================= WEBSOCKET HUB =================
	hub := notifications.NewHub()
	go hub.Run()

	// ================= WORKERS =================
	whatsapp := notifications.NewWhatsAppSender(
		whatsAppToken,
		phoneID,
	)

	go notifications.StartWhatsAppWorker(eventBus, whatsapp)
	go notifications.StartKitchenWorker(eventBus, hub)
	go workers.StartSessionExpiryWorker(queries)

	// ================= SERVICES =================
	tableSessionService := service.NewTableSessionService(queries)
	customerService := service.NewCustomerSessionService(queries)
	cartService := service.NewSharedCartService(queries)
	analyticsService := service.NewAnalyticsService(queries)
	orderService := service.NewOrderService(
		queries,
		eventBus,
	)

	billingService := service.NewBillingService(queries, eventBus)

	authService := service.NewAuthService(queries, []byte(jwtSecret))
	menuService := service.NewMenuService(queries)

	branchService := service.NewBranchService(queries)
	tableService := service.NewTableService(queries)

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

	// ================= ROUTER =================
	r := router.SetupRouter(
		dbpool,
		[]byte(jwtSecret),
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
	)

	// ================= SERVER =================
	log.Println("Server starting on :8080")

	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server failed:", err)
	}
}

// ================= HELPERS =================

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
