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

type SubmitCartRequest struct {
	TableSessionID    string `json:"table_session_id"`
	CustomerSessionID string `json:"customer_session_id"`
	CartID            string `json:"cart_id"`
}

func (h *OrderHandler) SubmitCart(c *gin.Context) {
	var req SubmitCartRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tableSessionID, err := uuid.Parse(req.TableSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table session id"})
		return
	}
	
	customerSessionID, err := uuid.Parse(req.CustomerSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer session id"})
		return
	}
	
	cartID, err := uuid.Parse(req.CartID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cart id"})
		return
	}

	order, err := h.Service.SubmitCart(
		c.Request.Context(),
		tableSessionID,
		customerSessionID,
		cartID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

type UpdateStatusRequest struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	err = h.Service.UpdateStatus(
		c.Request.Context(),
		orderID,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// Add this method for closing bills
type CloseBillRequest struct {
	OrderID string `json:"order_id"`
}

func (h *OrderHandler) CloseBill(c *gin.Context) {
	var req CloseBillRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	// Call the service method to close the bill
	err = h.Service.CloseBill(
		c.Request.Context(),
		orderID,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "bill closed"})
}
