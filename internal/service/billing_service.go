package service

import (
	"context"
	"log"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/notifications"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type BillingService struct {
	Queries  *db.Queries
	EventBus *notifications.EventBus
	Activity *ActivityService
}

func NewBillingService(
	q *db.Queries,
	eventBus *notifications.EventBus,
	activity *ActivityService,
) *BillingService {
	return &BillingService{
		Queries:  q,
		EventBus: eventBus,
		Activity: activity,
	}
}

func (s *BillingService) CloseBill(
	ctx context.Context,
	tableSessionID uuid.UUID,
) error {

	orders, err := s.Queries.GetOrdersBySession(ctx, tableSessionID)
	if err != nil {
		return err
	}

	var orderIDs []uuid.UUID

	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}

	// avoid empty slice issue
	if len(orderIDs) > 0 {
		pgIDs := make([]pgtype.UUID, len(orderIDs))
		for i, id := range orderIDs {
			pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
		}

		err = s.Queries.MarkOrdersPaid(ctx, pgIDs)
		if err != nil {
			return err
		}
	}

	err = s.Queries.CloseTableSession(ctx, tableSessionID)
	if err != nil {
		return err
	}

	// Log activity
	staffID, branchID := staffFromContext(ctx)
	s.Activity.Log(ctx, ActivityParams{
		StaffID:    staffID,
		BranchID:   branchID,
		Action:     "bill.close",
		EntityType: "table_session",
		EntityID:   tableSessionID,
	})

	// publish event AFTER successful DB operations
	event := map[string]string{
		"type":             "bill_closed",
		"table_session_id": tableSessionID.String(),
	}

	if err := s.EventBus.Publish("billing.closed", event); err != nil {
		log.Println("failed to publish billing event:", err)
	}

	return nil
}
