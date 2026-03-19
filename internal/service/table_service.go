package service

import (
	"context"

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
