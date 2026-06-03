package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type BranchContext struct {
	RestaurantName string
	BranchID       string
	GeneratedAt    time.Time

	TotalOrders     int64
	TotalRevenue    float64
	PendingOrders   int64
	PreparingOrders int64
	ReadyOrders     int64

	TopItems      []TopItem
	PeakHours     []PeakHour
	LowStockItems []LowStockItem
	StaffCount    int32
}

type TopItem struct {
	Name      string
	TotalSold int32
	Revenue   float64
}

type PeakHour struct {
	Hour       int32
	OrderCount int32
}

type LowStockItem struct {
	Name      string
	Stock     int32
	Threshold int32
}

func BuildBranchContext(
	ctx context.Context,
	q *db.Queries,
	tenantID uuid.UUID,
	branchID uuid.UUID,
	restaurantName string,
	logger *slog.Logger,
) (*BranchContext, error) {

	bc := &BranchContext{
		RestaurantName: restaurantName,
		BranchID:       branchID.String(),
		GeneratedAt:    time.Now(),
	}

	// Core metrics
	summary, err := q.GetAIBranchContext(ctx, branchID)
	if err != nil {
		logger.WarnContext(ctx, "ai: GetAIBranchContext failed",
			"branch_id", branchID, "error", err)
	} else {
		bc.TotalOrders = summary.TotalOrders
		bc.PendingOrders = summary.PendingOrders
		bc.PreparingOrders = summary.PreparingOrders
		bc.ReadyOrders = summary.ReadyOrders
		bc.TotalRevenue = numericToFloat64(summary.TotalRevenue)
	}

	// Top items
	topItems, err := q.GetAITopItems(ctx, branchID)
	if err != nil {
		logger.WarnContext(ctx, "ai: GetAITopItems failed",
			"branch_id", branchID, "error", err)
	} else {
		for _, item := range topItems {
			bc.TopItems = append(bc.TopItems, TopItem{
				Name:      item.Name,
				TotalSold: item.TotalSold,
				Revenue:   numericToFloat64(item.Revenue),
			})
		}
	}

	// Peak hours
	peaks, err := q.GetAIPeakHours(ctx, branchID)
	if err != nil {
		logger.WarnContext(ctx, "ai: GetAIPeakHours failed",
			"branch_id", branchID, "error", err)
	} else {
		for _, p := range peaks {
			bc.PeakHours = append(bc.PeakHours, PeakHour{
				Hour:       p.Hour,
				OrderCount: p.OrderCount,
			})
		}
	}

	// Low stock
	lowStock, err := q.GetAILowStockItems(ctx, branchID)
	if err != nil {
		logger.WarnContext(ctx, "ai: GetAILowStockItems failed",
			"branch_id", branchID, "error", err)
	} else {
		for _, item := range lowStock {
			bc.LowStockItems = append(bc.LowStockItems, LowStockItem{
				Name:      item.Name,
				Stock:     item.Stock,
				Threshold: item.Threshold,
			})
		}
	}

	// Staff count
	count, err := q.GetAIStaffCount(ctx, db.GetAIStaffCountParams{
		TenantID: tenantID,
		BranchID: pgtype.UUID{Bytes: branchID, Valid: true},
	})
	if err != nil {
		logger.WarnContext(ctx, "ai: GetAIStaffCount failed",
			"branch_id", branchID, "tenant_id", tenantID, "error", err)
	} else {
		bc.StaffCount = count
	}

	return bc, nil
}

func numericToFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case pgtype.Numeric:
		f, _ := val.Float64Value()
		return f.Float64
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

func (bc *BranchContext) FormatAsPromptContext() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("RESTAURANT: %s\n", bc.RestaurantName))
	b.WriteString(fmt.Sprintf("DATA PERIOD: Last 7 days (as of %s)\n\n",
		bc.GeneratedAt.Format("2006-01-02 15:04")))

	b.WriteString("ORDER SUMMARY:\n")
	b.WriteString(fmt.Sprintf("- Total orders: %d\n", bc.TotalOrders))
	b.WriteString(fmt.Sprintf("- Total revenue: KES %.0f\n", bc.TotalRevenue))
	b.WriteString(fmt.Sprintf("- Currently pending: %d | preparing: %d | ready: %d\n\n",
		bc.PendingOrders, bc.PreparingOrders, bc.ReadyOrders))

	if len(bc.TopItems) > 0 {
		b.WriteString("TOP SELLING ITEMS:\n")
		for i, item := range bc.TopItems {
			b.WriteString(fmt.Sprintf("%d. %s — %d sold, KES %.0f revenue\n",
				i+1, item.Name, item.TotalSold, item.Revenue))
		}
		b.WriteString("\n")
	}

	if len(bc.PeakHours) > 0 {
		b.WriteString("PEAK HOURS:\n")
		for _, ph := range bc.PeakHours {
			b.WriteString(fmt.Sprintf("- %02d:00 — %d orders\n", ph.Hour, ph.OrderCount))
		}
		b.WriteString("\n")
	}

	if len(bc.LowStockItems) > 0 {
		b.WriteString("LOW STOCK ALERTS:\n")
		for _, item := range bc.LowStockItems {
			b.WriteString(fmt.Sprintf("- %s: %d remaining (threshold: %d)\n",
				item.Name, item.Stock, item.Threshold))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("STAFF: %d members at this branch\n", bc.StaffCount))

	return b.String()
}
