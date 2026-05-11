package service

import (
	"context"
	"errors"
	"fmt"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrInternal = errors.New("internal service error")
)

type AnalyticsService struct {
	Queries *db.Queries
}

func NewAnalyticsService(q *db.Queries) *AnalyticsService {
	return &AnalyticsService{Queries: q}
}

func (s *AnalyticsService) GetDashboard(
	ctx context.Context,
	branchID uuid.UUID,
) (map[string]interface{}, error) {

	dailySales, err := s.Queries.GetDailySales(ctx, branchID)
	if err != nil {
		// Internal loggable context preserved with %w
		return nil, fmt.Errorf("%w: GetDailySales failed: %v", ErrInternal, err)
	}

	popularItems, err := s.Queries.GetPopularItems(ctx, branchID)
	if err != nil {
		return nil, fmt.Errorf("%w: GetPopularItems failed: %v", ErrInternal, err)
	}

	peakHours, err := s.Queries.GetPeakHours(ctx, branchID)
	if err != nil {
		return nil, fmt.Errorf("%w: GetPeakHours failed: %v", ErrInternal, err)
	}

	returningCustomers, err := s.Queries.GetReturningCustomers(ctx, branchID)
	if err != nil {
		return nil, fmt.Errorf("%w: GetReturningCustomers failed: %v", ErrInternal, err)
	}

	// Optional:
	// if len(dailySales) == 0 {
	//     return nil, ErrNotFound
	// }

	return map[string]interface{}{
		"daily_sales":         dailySales,
		"popular_items":       popularItems,
		"peak_hours":          peakHours,
		"returning_customers": returningCustomers,
	}, nil
}
