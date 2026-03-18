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
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/restaurant_saas")
	whatsAppToken := getEnv("WHATSAPP_TOKEN", "")
	phoneID := getEnv("WHATSAPP_PHONE_ID", "")

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

	orderService := service.NewOrderService(
		queries,
		eventBus,
	)

	billingService := service.NewBillingService(queries, eventBus)

	//authService := service.NewAuthService(queries)
	menuService := service.NewMenuService(queries)

	// ================= HANDLERS =================
	tableSessionHandler := handler.NewTableSessionHandler(tableSessionService)
	customerHandler := handler.NewCustomerSessionHandler(customerService)
	cartHandler := handler.NewSharedCartHandler(cartService)
	orderHandler := handler.NewOrderHandler(orderService)
	wsHandler := handler.NewWSHandler(hub)

	billingHandler := handler.NewBillingHandler(billingService)

	// (Assumed handlers — adjust if needed)
	//authHandler := handler.NewAuthHandler(authService)
	menuHandler := handler.NewMenuHandler(menuService)

	// ================= ROUTER =================
	r := router.SetupRouter(
		dbpool,
		//authHandler,
		tableSessionHandler,
		customerHandler,
		cartHandler,
		orderHandler,
		wsHandler,
		menuHandler,
		billingHandler,
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
