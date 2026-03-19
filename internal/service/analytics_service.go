package service

import (
	"context"

	db "mezzani_backend/internal/database/sqlc"
)

type AnalyticsService struct {
	Queries *db.Queries
}

func NewAnalyticsService(q *db.Queries) *AnalyticsService {
	return &AnalyticsService{Queries: q}
}

func (s *AnalyticsService) GetDashboard(ctx context.Context) (map[string]interface{}, error) {

	dailySales, _ := s.Queries.GetDailySales(ctx)
	popularItems, _ := s.Queries.GetPopularItems(ctx)
	peakHours, _ := s.Queries.GetPeakHours(ctx)
	returningCustomers, _ := s.Queries.GetReturningCustomers(ctx)

	return map[string]interface{}{
		"daily_sales":         dailySales,
		"popular_items":       popularItems,
		"peak_hours":          peakHours,
		"returning_customers": returningCustomers,
	}, nil
}
