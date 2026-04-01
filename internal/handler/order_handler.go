package handler

import (
	"log"
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

//
// =========================
// REQUEST STRUCTS
// =========================
//

type SubmitCartRequest struct {
	TableSessionID    string `json:"table_session_id"    binding:"required,uuid"`
	CustomerSessionID string `json:"customer_session_id" binding:"required,uuid"`
	CartID            string `json:"cart_id"             binding:"required,uuid"`
}

//
// =========================
// SUBMIT CART
// =========================
//

func (h *OrderHandler) SubmitCart(c *gin.Context) {
	var req SubmitCartRequest

	//  Validate request body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//  Parse ALL UUIDs safely (no ignoring errors)
	tableSessionID, err := uuid.Parse(req.TableSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table_session_id"})
		return
	}

	customerSessionID, err := uuid.Parse(req.CustomerSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_session_id"})
		return
	}

	cartID, err := uuid.Parse(req.CartID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cart_id"})
		return
	}

	log.Printf(
		"Submitting order | table_session=%s | customer_session=%s | cart=%s",
		tableSessionID,
		customerSessionID,
		cartID,
	)

	// 🚀 Call service
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

//
// =========================
// UPDATE STATUS
// =========================
//

type UpdateStatusRequest struct {
	OrderID string `json:"order_id" binding:"required,uuid"`
	Status  string `json:"status"   binding:"required"`
}

func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest

	//  Validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//  Validate status
	if !validOrderStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid status, must be one of: accepted, preparing, ready, served, paid",
		})
		return
	}

	//  Parse UUID safely
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id"})
		return
	}

	log.Printf(
		"Updating order status | order=%s | status=%s",
		orderID,
		req.Status,
	)

	//  Update
	if err := h.Service.UpdateStatus(c.Request.Context(), orderID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
