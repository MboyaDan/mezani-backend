package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/domain"
	"mezzani_backend/internal/notifications"

	"github.com/google/uuid"
)

// maxOrderNoteLen caps a guest's free-text note (characters, not bytes).
const maxOrderNoteLen = 300

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
	note string,
) (db.Order, error) {

	// Guest note: trimmed and capped (the handler also validates max 300 characters).
	note = strings.TrimSpace(note)
	if r := []rune(note); len(r) > maxOrderNoteLen {
		note = string(r[:maxOrderNoteLen])
	}

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
		Note:              note,
	})
	if err != nil {
		return db.Order{}, err
	}

	// Only log activity if a real staff member is in context.
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

	// Fetch table number AND branch_id for kitchen routing.
	var tableNumber int32
	var branchIDForKDS uuid.UUID

	table, err := s.Queries.GetTable(ctx, session.TableID)
	if err != nil {
		// FIX (Silent error — SubmitCart): Failing to resolve the table means we
		// cannot route the KDS event to the correct branch. Rather than silently
		// publishing an event with a zero BranchID (which the worker would either
		// drop or misroute), return an error so the caller can retry or surface the
		// problem. The order row has already been created; callers may choose to
		// wrap this whole function in a transaction if atomicity is required.
		return db.Order{}, fmt.Errorf("order created but failed to resolve table for KDS routing (tableID %s): %w", session.TableID, err)
	}
	tableNumber = table.TableNumber
	branchIDForKDS = table.BranchID

	log.Println("KDS EVENT BRANCH:", branchIDForKDS)

	event := notifications.KDSOrderCreatedEvent{
		Type:        "new_order",
		OrderID:     order.ID.String(),
		BranchID:    branchIDForKDS.String(),
		TableID:     tableSessionID.String(),
		TableNumber: tableNumber,
		Items:       items,
		Note:        note,
	}

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

	// Only log activity if a real staff member is in context.
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

	// Deduct stock when order is confirmed.
	if newStatus == "confirmed" {
		if err := s.Inventory.DeductForOrder(ctx, orderID, branchID); err != nil {
			log.Println("stock deduction failed:", err)
		}
	}

	// Restore stock when a confirmed order is cancelled.
	if newStatus == "cancelled" && string(current) == "confirmed" {
		if err := s.Inventory.RestoreForOrder(ctx, orderID, branchID); err != nil {
			log.Println("stock restore failed:", err)
		}
	}

	// FIX (Silent error — UpdateStatus): When no staff context is present (e.g.
	// a customer-triggered update), we must resolve branchID from the DB to route
	// the KDS event correctly. The original code silently swallowed both the
	// GetTableSession and GetTable errors, meaning a DB failure would publish an
	// event with a zero BranchID — causing the kitchen display worker to silently
	// drop or misroute the update.
	//
	// Now we return a descriptive error so the caller knows the status was updated
	// but the KDS notification could not be dispatched. Operators can decide
	// whether to retry or alert.
	if branchID == uuid.Nil {
		session, err := s.Queries.GetTableSession(ctx, updatedOrder.TableSessionID)
		if err != nil {
			return fmt.Errorf(
				"order status updated but failed to resolve table session for KDS routing (tableSessionID %s): %w",
				updatedOrder.TableSessionID, err,
			)
		}

		table, err := s.Queries.GetTable(ctx, session.TableID)
		if err != nil {
			return fmt.Errorf(
				"order status updated but failed to resolve table for KDS routing (tableID %s): %w",
				session.TableID, err,
			)
		}

		branchID = table.BranchID
	}

	event := notifications.KDSOrderStatusUpdatedEvent{
		Type:     "order_updated",
		OrderID:  updatedOrder.ID.String(),
		BranchID: branchID.String(),
		Status:   updatedOrder.Status,
	}

	if err := s.EventBus.Publish("orders.status", event); err != nil {
		log.Printf("Warning: failed to publish KDS order.status event: %v", err)
	}

	return nil
}

// staffFromContext pulls staff_id and branch_id from the auth context.
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
	ID             string      `json:"id"`
	TableSessionID string      `json:"table_session_id"`
	TableNumber    int32       `json:"table_number"`
	Status         string      `json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
	Items          []OrderItem `json:"items"`
	Total          float64     `json:"total"`
	Note           string      `json:"note"`
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
			ID:             o.ID.String(),
			TableSessionID: o.TableSessionID.String(),
			TableNumber:    o.TableNumber,
			Status:         o.Status,
			CreatedAt:      o.CreatedAt,
			Items:          orderItems,
			Total:          total,
			Note:           o.Note,
		})
	}
	return result, nil
}
