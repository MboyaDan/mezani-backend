package main

import (
	"context"
	"log"

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

	// ================= DATABASE =================
	dbpool, err := pgxpool.New(
		context.Background(),
		"postgres://postgres:postgres@localhost:5432/restaurant_saas",
	)
	if err != nil {
		log.Fatal(err)
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
		cfg.WhatsAppToken,
		cfg.PhoneID,
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
		eventBus, // inject event bus
	)

	// ================= HANDLERS =================

	tableSessionHandler := handler.NewTableSessionHandler(tableSessionService)

	customerHandler := handler.NewCustomerSessionHandler(customerService)

	cartHandler := handler.NewSharedCartHandler(cartService)

	orderHandler := handler.NewOrderHandler(orderService)

	wsHandler := handler.NewWSHandler(hub)

	// ================= ROUTER =================

	r := router.SetupRouter(
		tableSessionHandler,
		customerHandler,
		cartHandler,
		orderHandler,
		wsHandler,
	)

	// ================= SERVER =================

	log.Println("Server starting on :8080")

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
