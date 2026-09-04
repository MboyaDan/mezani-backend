package service

import (
	"context"
	"errors"
	"fmt"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrBranchLimitReached = errors.New("branch limit reached for current plan")

type BranchService struct {
	Queries *db.Queries
}

func NewBranchService(q *db.Queries) *BranchService {
	return &BranchService{Queries: q}
}

// CreateBranch enforces the tenant's plan branch limit atomically. The
// count-check-then-insert sequence runs inside a single transaction with
// the tenant row locked (SELECT ... FOR UPDATE) for its duration — this
// is what actually closes the race, not just wrapping things in a
// transaction: without the row lock, two concurrent transactions could
// each independently see "2 of 3 branches used" and both proceed to
// insert, landing at 4. With the lock, the second transaction blocks
// until the first commits or rolls back, at which point its own count
// correctly reflects the branch the first one just created.
func (s *BranchService) CreateBranch(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
	location string,
) (db.Branch, error) {

	tx, err := s.Queries.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Branch{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	qtx := s.Queries.WithTx(tx)

	tenant, err := qtx.GetTenantByIDForUpdate(ctx, tenantID)
	if err != nil {
		return db.Branch{}, fmt.Errorf("failed to look up tenant: %w", err)
	}

	plan, err := qtx.GetPlanByName(ctx, tenant.Plan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Branch{}, fmt.Errorf("tenant's plan %q is not a recognized plan", tenant.Plan)
		}
		return db.Branch{}, fmt.Errorf("failed to look up plan: %w", err)
	}

	currentCount, err := qtx.CountBranchesByTenant(ctx, tenantID)
	if err != nil {
		return db.Branch{}, fmt.Errorf("failed to count existing branches: %w", err)
	}

	if currentCount >= plan.MaxBranches {
		return db.Branch{}, ErrBranchLimitReached
	}

	branch, err := qtx.CreateBranch(ctx, db.CreateBranchParams{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     name,
		Location: location,
	})
	if err != nil {
		return db.Branch{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Branch{}, fmt.Errorf("failed to commit branch creation: %w", err)
	}

	return branch, nil
}

func (s *BranchService) GetBranchByID(ctx context.Context, branchID uuid.UUID) (db.Branch, error) {
	return s.Queries.GetBranchByID(ctx, branchID)
}
func (s *BranchService) GetBranchesByTenant(ctx context.Context, tenantID uuid.UUID) ([]db.Branch, error) {
	return s.Queries.GetBranchesByTenant(ctx, tenantID)
}

func (s *BranchService) DeleteBranch(ctx context.Context, id uuid.UUID) error {
	return s.Queries.DeleteBranch(ctx, id)
}
