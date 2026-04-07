package service

import (
	"context"
	"errors"
	"log"
	"time"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/domain"
	"mezzani_backend/internal/notifications"

	"github.com/google/uuid"
)

type OrderService struct {
	Queries   *db.Queries
	EventBus  *notifications.EventBus
	Activity  *ActivityService
	Inventory *InventoryService
}

func NewOrderService(
	q *db.Queries,
	bus *notifications.EventBus,
	activity *ActivityService,
	inventory *InventoryService,
) *OrderService {
	return &OrderService{
		Queries:   q,
		EventBus:  bus,
		Activity:  activity,
		Inventory: inventory,
	}
}

// Submit Cart → Create Order → Send to Kitchen
func (s *OrderService) SubmitCart(
	ctx context.Context,
	tableSessionID uuid.UUID,
	customerSessionID uuid.UUID,
	cartID uuid.UUID,
) (db.Order, error) {

	session, err := s.Queries.GetTableSession(ctx, tableSessionID)
	if err != nil {
		return db.Order{}, err
	}

	if session.Status != "active" {
		return db.Order{}, errors.New("table session is closed")
	}

	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		return db.Order{}, errors.New("table session expired")
	}

	order, err := s.Queries.CreateOrder(ctx, db.CreateOrderParams{
		ID:                uuid.New(),
		TableSessionID:    tableSessionID,
		CustomerSessionID: customerSessionID,
		CartID:            cartID,
	})
	if err != nil {
		return db.Order{}, err
	}

	// Only log activity if a real staff member is in context
	staffID, branchID := staffFromContext(ctx)
	if staffID != uuid.Nil {
		s.Activity.Log(ctx, ActivityParams{
			StaffID:    staffID,
			BranchID:   branchID,
			Action:     "order.create",
			EntityType: "order",
			EntityID:   order.ID,
		})
	}

	cartItems, err := s.Queries.GetCartItems(ctx, cartID)
	if err != nil {
		return db.Order{}, err
	}

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

	if err := s.Queries.ClearCartItems(ctx, cartID); err != nil {
		return db.Order{}, err
	}

	// Fetch table number for kitchen display
	var tableNumber int32
	table, err := s.Queries.GetTable(ctx, session.TableID)
	if err == nil {
		tableNumber = table.TableNumber
	}

	event := notifications.KDSOrderCreatedEvent{
		Type:        "new_order",
		OrderID:     order.ID.String(),
		TableID:     tableSessionID.String(),
		TableNumber: tableNumber,
		Items:       items,
	}

	// Log publish errors instead of silently swallowing them
	if err := s.EventBus.Publish("orders.new", event); err != nil {
		log.Printf("Warning: failed to publish KDS order.create event: %v", err)
	}

	return order, nil
}

// Get Orders by Table
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
		latest, err2 := s.Queries.GetOrderByID(ctx, orderID)
		if err2 != nil {
			return err
		}

		if latest.Status == newStatus {
			return nil // safe retry
		}

		return errors.New("order status changed by another process")
	}

	//  Only log activity if a real staff member is in context
	staffID, branchID := staffFromContext(ctx)
	if staffID != uuid.Nil {
		s.Activity.Log(ctx, ActivityParams{
			StaffID:    staffID,
			BranchID:   branchID,
			Action:     "order.status_update",
			EntityType: "order",
			EntityID:   updatedOrder.ID,
			OldData:    map[string]any{"status": string(current)},
			NewData:    map[string]any{"status": newStatus},
		})
	}

	// Deduct stock when order is confirmed
	if newStatus == "confirmed" {
		if err := s.Inventory.DeductForOrder(ctx, orderID, branchID); err != nil {
			log.Println("stock deduction failed:", err)
			// log only — don't fail the order over stock tracking
		}
	}

	// Restore stock when a confirmed order is cancelled
	if newStatus == "cancelled" && string(current) == "confirmed" {
		if err := s.Inventory.RestoreForOrder(ctx, orderID, branchID); err != nil {
			log.Println("stock restore failed:", err)
		}
	}

	// TO:
	event := notifications.KDSOrderStatusUpdatedEvent{
		Type:    "order_updated",
		OrderID: updatedOrder.ID.String(),
		Status:  updatedOrder.Status,
	}

	if err := s.EventBus.Publish("orders.status", event); err != nil {
		log.Printf("Warning: failed to publish KDS order.status event: %v", err)
	}

	return nil
}

// staffFromContext pulls staff_id and branch_id from the auth context
func staffFromContext(ctx context.Context) (staffID uuid.UUID, branchID uuid.UUID) {
	if id, ok := ctx.Value("user_id").(string); ok {
		staffID, _ = uuid.Parse(id)
	}
	if id, ok := ctx.Value("branch_id").(string); ok {
		branchID, _ = uuid.Parse(id)
	}
	return
}

type OrderWithItems struct {
    ID          string      `json:"id"`
    TableNumber int32       `json:"table_number"`
    Status      string      `json:"status"`
    CreatedAt   time.Time   `json:"created_at"`
    Items       []OrderItem `json:"items"`
    Total       float64     `json:"total"`
}

type OrderItem struct {
    Name     string  `json:"name"`
    Quantity int32   `json:"quantity"`
    Price    float64 `json:"price"`
}

func (s *OrderService) GetRecentOrders(ctx context.Context, branchID uuid.UUID) ([]OrderWithItems, error) {
    orders, err := s.Queries.GetRecentOrdersByBranch(ctx, branchID)
    if err != nil {
        return nil, err
    }

    result := make([]OrderWithItems, 0, len(orders))
    for _, o := range orders {
        items, err := s.Queries.GetOrderItemsByOrder(ctx, o.ID)
        if err != nil {
            continue
        }

        var total float64
        orderItems := make([]OrderItem, 0, len(items))
        for _, item := range items {
            orderItems = append(orderItems, OrderItem{
                Name:     item.Name,
                Quantity: item.Quantity,
                Price:    item.Price,
            })
            total += item.Price * float64(item.Quantity)
        }

        result = append(result, OrderWithItems{
            ID:          o.ID.String(),
            TableNumber: o.TableNumber,
            Status:      o.Status,
            CreatedAt:   o.CreatedAt,
            Items:       orderItems,
            Total:       total,
        })
    }
    return result, nil
}