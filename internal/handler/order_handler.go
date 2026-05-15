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

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	// No hub needed here — service publishes to EventBus,
	// StartKitchenWorker picks it up and routes to the right branch via BroadcastToBranch
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

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validOrderStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid status, must be one of: accepted, preparing, ready, served, paid",
		})
		return
	}

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

	//  No hub needed here — service publishes to EventBus,
	// StartKitchenWorker picks it up and routes to the right branch via BroadcastToBranch
	if err := h.Service.UpdateStatus(c.Request.Context(), orderID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

//
// =========================
// GET RECENT ORDERS
// =========================
//

func (h *OrderHandler) GetRecentOrders(c *gin.Context) {
	branchIDStr, exists := c.Get("branch_id")
	if !exists {
		c.JSON(400, gin.H{"error": "branch_id missing from token"})
		return
	}

	// Owner has no branch_id in token — check query param
	branchIDRaw := branchIDStr.(string)
	if branchIDRaw == "00000000-0000-0000-0000-000000000000" || branchIDRaw == "" {
		branchIDRaw = c.Query("branch_id")
	}

	branchID, err := uuid.Parse(branchIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid branch_id"})
		return
	}

	orders, err := h.Service.GetRecentOrders(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, orders)
}
