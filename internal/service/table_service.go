package service

import (
	"context"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type TableService struct {
	Queries *db.Queries
}

func NewTableService(q *db.Queries) *TableService {
	return &TableService{Queries: q}
}

func (s *TableService) CreateTable(
	ctx context.Context,
	branchID uuid.UUID,
	tableNumber int32,
) (db.Table, error) {

	return s.Queries.CreateTable(ctx, db.CreateTableParams{
		ID:          uuid.New(),
		BranchID:    branchID,
		TableNumber: tableNumber,
	})
}

type TableWithSession struct {
	ID          string     `json:"id"`
	TableNumber int32      `json:"table_number"`
	BranchID    string     `json:"branch_id"`
	Status      string     `json:"status"` // free, active, expiring
	SessionID   *string    `json:"session_id,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

func (s *TableService) GetTablesWithSessions(ctx context.Context, branchID uuid.UUID) ([]TableWithSession, error) {
	tables, err := s.Queries.GetTablesByBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}

	result := make([]TableWithSession, 0, len(tables))
	for _, t := range tables {
		tw := TableWithSession{
			ID:          t.ID.String(),
			TableNumber: t.TableNumber,
			BranchID:    t.BranchID.String(),
			Status:      "free",
		}

		session, err := s.Queries.GetActiveTableSessionByTable(ctx, t.ID)
		if err == nil {
			sessionID := session.ID.String()
			tw.SessionID = &sessionID
			tw.ExpiresAt = &session.ExpiresAt
			tw.CreatedAt = &session.CreatedAt

			// Determine if expiring — less than 15 minutes left
			minsLeft := time.Until(session.ExpiresAt).Minutes()
			if minsLeft <= 15 {
				tw.Status = "expiring"
			} else {
				tw.Status = "active"
			}
		}

		result = append(result, tw)
	}
	return result, nil
}
