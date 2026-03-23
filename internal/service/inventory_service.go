package service

import (
	"context"
	"log"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type InventoryService struct {
	Queries *db.Queries
}

func NewInventoryService(q *db.Queries) *InventoryService {
	return &InventoryService{Queries: q}
}

func (s *InventoryService) CreateItem(
	ctx context.Context,
	branchID uuid.UUID,
	name string,
	stock int32,
	threshold int32,
) (db.InventoryItem, error) {
	return s.Queries.CreateInventoryItem(ctx, db.CreateInventoryItemParams{
		ID:        uuid.New(),
		BranchID:  branchID,
		Name:      name,
		Stock:     stock,
		Threshold: threshold,
	})
}

func (s *InventoryService) GetByBranch(
	ctx context.Context,
	branchID uuid.UUID,
) ([]db.InventoryItem, error) {
	return s.Queries.GetInventoryItemsByBranch(ctx, branchID)
}

func (s *InventoryService) GetByID(
	ctx context.Context,
	id uuid.UUID,
	branchID uuid.UUID,
) (db.InventoryItem, error) {
	return s.Queries.GetInventoryItemByID(ctx, db.GetInventoryItemByIDParams{
		ID:       id,
		BranchID: branchID,
	})
}

// SetStock → manual correction after physical count
func (s *InventoryService) SetStock(
	ctx context.Context,
	id uuid.UUID,
	branchID uuid.UUID,
	newStock int32,
) (db.InventoryItem, error) {
	return s.Queries.SetStock(ctx, db.SetStockParams{
		ID:       id,
		BranchID: branchID,
		Stock:    newStock,
	})
}

func (s *InventoryService) GetLowStock(
	ctx context.Context,
	branchID uuid.UUID,
) ([]db.InventoryItem, error) {
	return s.Queries.GetLowStockItems(ctx, branchID)
}

// STOCK DEDUCTION — called by OrderService on confirmation

func (s *InventoryService) DeductForOrder(
	ctx context.Context,
	orderID uuid.UUID,
	branchID uuid.UUID,
) error {
	// Single query — deducts all ingredients for all items in one round trip
	if err := s.Queries.DeductStockForOrder(ctx, db.DeductStockForOrderParams{
		OrderID:  orderID,
		BranchID: branchID,
	}); err != nil {
		return err
	}

	// Auto-flag any dishes that are now unbuildable
	return s.Queries.AutoMarkSoldOut(ctx, branchID)
}

// RestoreForOrder → called when a confirmed order is cancelled
func (s *InventoryService) RestoreForOrder(
	ctx context.Context,
	orderID uuid.UUID,
	branchID uuid.UUID,
) error {
	return s.Queries.RestoreStockForOrder(ctx, db.RestoreStockForOrderParams{
		OrderID:  orderID,
		BranchID: branchID,
	})
}

// BACKGROUND WORKER — low stock alerts

func StartInventoryWorker(q *db.Queries, branchIDs []uuid.UUID) {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		for _, branchID := range branchIDs {
			items, err := q.GetLowStockItems(context.Background(), branchID)
			if err != nil {
				log.Println("inventory worker error:", err)
				continue
			}
			for _, item := range items {
				log.Printf("LOW STOCK [branch: %s] %s — %d remaining\n",
					branchID, item.Name, item.Stock)
				// Later → WhatsApp alert to manager with "Restock" button that links to inventory management UI
				// (can be implemented as a deep link with a custom URL scheme that opens the app to the right screen)
			}
		}
	}
}
