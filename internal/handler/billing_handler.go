package handler

import (
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BillingHandler struct {
	Service *service.BillingService
}

func NewBillingHandler(s *service.BillingService) *BillingHandler {
	return &BillingHandler{Service: s}
}

type CloseBillRequest struct {
	TableSessionID string `json:"table_session_id" binding:"required,uuid"`
}

func (h *BillingHandler) CloseBill(c *gin.Context) {
	var req CloseBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	sessionID, err := uuid.Parse(req.TableSessionID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid session id"})
		return
	}
	if err := h.Service.CloseBill(c.Request.Context(), sessionID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "bill closed"})
}
