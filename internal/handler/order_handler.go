package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type OrderHandler struct {
	Service *service.OrderService
}

func NewOrderHandler(s *service.OrderService) *OrderHandler {
	return &OrderHandler{Service: s}
}

var validOrderStatuses = map[string]bool{
	"accepted":  true,
	"preparing": true,
	"ready":     true,
	"served":    true,
	"paid":      true,
}

type SubmitCartRequest struct {
	TableSessionID    string `json:"table_session_id"    binding:"required,uuid"`
	CustomerSessionID string `json:"customer_session_id" binding:"required,uuid"`
	CartID            string `json:"cart_id"             binding:"required,uuid"`
}

func (h *OrderHandler) SubmitCart(c *gin.Context) {
	var req SubmitCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tableSessionID, _ := uuid.Parse(req.TableSessionID)
	customerSessionID, _ := uuid.Parse(req.CustomerSessionID)
	cartID, _ := uuid.Parse(req.CartID)
	order, err := h.Service.SubmitCart(c.Request.Context(), tableSessionID, customerSessionID, cartID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

type UpdateStatusRequest struct {
	OrderID string `json:"order_id" binding:"required,uuid"`
	Status  string `json:"status"   binding:"required"`
}

func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validOrderStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status, must be one of: accepted, preparing, ready, served, paid"})
		return
	}
	orderID, _ := uuid.Parse(req.OrderID)
	if err := h.Service.UpdateStatus(c.Request.Context(), orderID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
