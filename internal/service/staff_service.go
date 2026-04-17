package service

import (
	"context"

	"github.com/google/uuid"

	db "mezzani_backend/internal/database/sqlc"
)

type StaffService struct {
	Queries *db.Queries
}

func NewStaffService(q *db.Queries) *StaffService {
	return &StaffService{Queries: q}
}

func (s *StaffService) GetStaffByBranch(ctx context.Context, branchID uuid.UUID) ([]db.StaffUser, error) {
	return s.Queries.GetBranchStaff(ctx, uuidToPgtype(branchID))
}

func (s *StaffService) DeleteStaff(ctx context.Context, staffID uuid.UUID) error {
	return s.Queries.DeleteStaffUser(ctx, staffID)
}
