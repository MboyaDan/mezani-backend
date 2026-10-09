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

// GetDashboard returns the analytics dashboard for a branch.
//
// days is the length of the reporting window in Nairobi calendar days
// (1 = today, 7, 30). "Today", the hour of each order and the previous-period
// comparison are all computed in Africa/Nairobi time; timestamps are stored as UTC.
func (s *AnalyticsService) GetDashboard(
	ctx context.Context,
	branchID uuid.UUID,
	days int32,
) (map[string]interface{}, error) {

	dailySales, err := s.Queries.GetDailySales(ctx, branchID)
	if err != nil {
		// Internal loggable context preserved with %w
		return nil, fmt.Errorf("%w: GetDailySales failed: %v", ErrInternal, err)
	}

	current, err := s.Queries.GetSalesSummary(ctx, db.GetSalesSummaryParams{
		BranchID: branchID, Days: days, Period: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: GetSalesSummary(current) failed: %v", ErrInternal, err)
	}

	previous, err := s.Queries.GetSalesSummary(ctx, db.GetSalesSummaryParams{
		BranchID: branchID, Days: days, Period: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: GetSalesSummary(previous) failed: %v", ErrInternal, err)
	}

	popularItems, err := s.Queries.GetPopularItems(ctx, db.GetPopularItemsParams{
		BranchID: branchID, Days: days,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: GetPopularItems failed: %v", ErrInternal, err)
	}

	peakHours, err := s.Queries.GetPeakHours(ctx, db.GetPeakHoursParams{
		BranchID: branchID, Days: days,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: GetPeakHours failed: %v", ErrInternal, err)
	}

	payments, err := s.Queries.GetPaymentsByMethod(ctx, db.GetPaymentsByMethodParams{
		BranchID: branchID, Days: days,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: GetPaymentsByMethod failed: %v", ErrInternal, err)
	}
	if payments == nil {
		payments = []db.GetPaymentsByMethodRow{} // JSON [] rather than null
	}

	returningCustomers, err := s.Queries.GetReturningCustomers(ctx, branchID)
	if err != nil {
		return nil, fmt.Errorf("%w: GetReturningCustomers failed: %v", ErrInternal, err)
	}

	avgOrderValue := 0.0
	if current.OrderCount > 0 {
		avgOrderValue = current.TotalSales / float64(current.OrderCount)
	}

	return map[string]interface{}{
		// Unchanged keys (the overview page still reads these).
		"daily_sales":         dailySales,
		"popular_items":       popularItems,
		"peak_hours":          peakHours,
		"returning_customers": returningCustomers,

		// New: range-aware figures for the analytics page.
		"range_days": days,
		"sales": map[string]interface{}{
			"total":           current.TotalSales,
			"orders":          current.OrderCount,
			"avg_order_value": avgOrderValue,
			"previous_total":  previous.TotalSales,
			"previous_orders": previous.OrderCount,
		},
		"payments_by_method": payments,
	}, nil
}
