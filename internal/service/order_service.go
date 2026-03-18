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
// Create order
	order, err := s.Queries.CreateOrder(ctx, db.CreateOrderParams{
		ID:                uuid.New(),
		TableSessionID:    tableSessionID,
		CustomerSessionID: customerSessionID,
		CartID:            cartID,
	})
	if err != nil {
		return db.Order{}, err
	}
// Get cart items and create order items
	cartItems, err := s.Queries.GetCartItems(ctx, cartID)
	if err != nil {
		return db.Order{}, err
	}

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
	}
	// Clear cart

	err = s.Queries.ClearCartItems(ctx, cartID)
	if err != nil {
		return db.Order{}, err
	}

	event := notifications.OrderCreatedEvent{
		OrderID:      order.ID.String(),
		TableSession: tableSessionID.String(),
	}

	err = s.EventBus.Publish("orders.new", event)
	if err != nil {
		// log but don't fail request
	}

	return order, nil
}

func (s *OrderService) GetOrdersByTable(
	ctx context.Context,
	tableSessionID uuid.UUID,
) ([]db.Order, error) {

	return s.Queries.GetOrdersByTableSession(ctx, tableSessionID)
}

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

		// idempotency handling
		latest, err2 := s.Queries.GetOrderByID(ctx, orderID)
		if err2 != nil {
			return err
		}

		if latest.Status == newStatus {
			// already updated → safe retry
			return nil
		}

		return errors.New("order status changed by another process")
	}

	event := notifications.OrderStatusUpdatedEvent{
		OrderID: updatedOrder.ID.String(),
		Status:  updatedOrder.Status,
	}

	err = s.EventBus.Publish("orders.status", event)
	if err != nil {
		// log but don't fail request
	}

	return nil
}
