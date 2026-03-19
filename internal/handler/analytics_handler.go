package handler

import (
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	Service *service.AnalyticsService
}

func NewAnalyticsHandler(s *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{Service: s}
}

func (h *AnalyticsHandler) Dashboard(c *gin.Context) {

	data, err := h.Service.GetDashboard(c.Request.Context())

	if err != nil {
		c.JSON(500, err)
		return
	}

	c.JSON(200, data)
}
