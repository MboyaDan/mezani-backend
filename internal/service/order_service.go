package service

import (
	"context"
	"errors"
	"time"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/domain"
	"mezzani_backend/internal/notifications"

	"github.com/google/uuid"
)

type OrderService struct {
	Queries  *db.Queries
	EventBus *notifications.EventBus
}

func NewOrderService(q *db.Queries, bus *notifications.EventBus) *OrderService {
	return &OrderService{
		Queries:  q,
		EventBus: bus,
	}
}

// Submit Cart → Create Order → Send to Kitchen
func (s *OrderService) SubmitCart(
	ctx context.Context,
	tableSessionID uuid.UUID,
	customerSessionID uuid.UUID,
	cartID uuid.UUID,
) (db.Order, error) {

	// Validate table session
	session, err := s.Queries.GetTableSession(ctx, tableSessionID)
	if err != nil {
		return db.Order{}, err
	}

	if session.Status != "active" {
		return db.Order{}, errors.New("table session is closed")
	}

	if time.Now().After(session.ExpiresAt) {
		return db.Order{}, errors.New("table session expired")
	}

	//  Create order
	order, err := s.Queries.CreateOrder(ctx, db.CreateOrderParams{
		ID:                uuid.New(),
		TableSessionID:    tableSessionID,
		CustomerSessionID: customerSessionID,
		CartID:            cartID,
	})
	if err != nil {
		return db.Order{}, err
	}

	// Get cart items (WITH names now)
	cartItems, err := s.Queries.GetCartItems(ctx, cartID)
	if err != nil {
		return db.Order{}, err
	}

	//  Create order items + build KDS payload
	var items []notifications.Item

	for _, item := range cartItems {

		_, err := s.Queries.CreateOrderItem(ctx, db.CreateOrderItemParams{
			ID:         uuid.New(),
			OrderID:    order.ID,
			MenuItemID: item.MenuItemID,
			Quantity:   item.Quantity,
		})
		if err != nil {
			return db.Order{}, err
		}

		items = append(items, notifications.Item{
			Name:     item.Name,
			Quantity: item.Quantity,
		})
	}

	//  Clear cart
	if err := s.Queries.ClearCartItems(ctx, cartID); err != nil {
		return db.Order{}, err
	}

	// Publish to Redis (KDS)
	event := notifications.KDSOrderCreatedEvent{
		OrderID: order.ID.String(),
		TableID: tableSessionID.String(),
		Items:   items,
	}

	if err := s.EventBus.Publish("orders.new", event); err != nil {
		// log only, don't fail request
	}

	return order, nil
}

// Get Orders
func (s *OrderService) GetOrdersByTable(
	ctx context.Context,
	tableSessionID uuid.UUID,
) ([]db.Order, error) {
	return s.Queries.GetOrdersByTableSession(ctx, tableSessionID)
}

// Update Order Status → Notify Kitchen
func (s *OrderService) UpdateStatus(
	ctx context.Context,
	orderID uuid.UUID,
	newStatus string,
) error {

	order, err := s.Queries.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	current := domain.OrderStatus(order.Status)
	next := domain.OrderStatus(newStatus)

	if !domain.CanTransition(current, next) {
		return errors.New("invalid order status transition")
	}

	updatedOrder, err := s.Queries.UpdateOrderStatusSafe(
		ctx,
		db.UpdateOrderStatusSafeParams{
			ID:       orderID,
			Status:   string(current),
			Status_2: newStatus,
		},
	)
	if err != nil {

		//  idempotency handling
		latest, err2 := s.Queries.GetOrderByID(ctx, orderID)
		if err2 != nil {
			return err
		}

		if latest.Status == newStatus {
			return nil // safe retry
		}

		return errors.New("order status changed by another process")
	}

	//  Publish status update
	event := notifications.KDSOrderStatusUpdatedEvent{
		OrderID: updatedOrder.ID.String(),
		Status:  updatedOrder.Status,
	}

	if err := s.EventBus.Publish("orders.status", event); err != nil {
		// log only
	}

	return nil
}
