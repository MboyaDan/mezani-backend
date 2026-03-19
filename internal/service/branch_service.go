package service

import (
	"context"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type BranchService struct {
	Queries *db.Queries
}

func NewBranchService(q *db.Queries) *BranchService {
	return &BranchService{Queries: q}
}

func (s *BranchService) CreateBranch(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
	location string,
) (db.Branch, error) {

	return s.Queries.CreateBranch(ctx, db.CreateBranchParams{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     name,
		Location: location,
	})
}
