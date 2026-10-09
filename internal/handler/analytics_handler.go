package handler

import (
	"errors"
	"net/http"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	Service *service.AnalyticsService
}

func NewAnalyticsHandler(s *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{Service: s}
}

// parseRangeDays maps the ?range= query value to a window length in days.
// Empty defaults to 30 days.
func parseRangeDays(r string) (int32, bool) {
	switch r {
	case "", "30d":
		return 30, true
	case "7d":
		return 7, true
	case "today":
		return 1, true
	}
	return 0, false
}

func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	branchID, err := uuid.Parse(c.Query("branch_id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid or missing branch_id",
		})
		return
	}

	days, ok := parseRangeDays(c.Query("range"))
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "range must be one of: today, 7d, 30d",
		})
		return
	}

	data, err := h.Service.GetDashboard(c.Request.Context(), branchID, days)
	if err != nil {

		// TODO: add structured logging
		// log.Printf("dashboard error: %v", err)

		switch {
		case errors.Is(err, service.ErrNotFound):
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "dashboard not found",
			})

		case errors.Is(err, service.ErrInternal):
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "failed to retrieve dashboard data",
			})

		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "unexpected server error",
			})
		}

		return
	}

	c.JSON(http.StatusOK, data)
}
