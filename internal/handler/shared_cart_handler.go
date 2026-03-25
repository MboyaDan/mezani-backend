package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type SharedCartHandler struct {
	Service *service.SharedCartService
}

func NewSharedCartHandler(s *service.SharedCartService) *SharedCartHandler {
	return &SharedCartHandler{Service: s}
}

type CreateCartRequest struct {
	TableSessionID string `json:"table_session_id" binding:"required,uuid"`
	CustomerID     string `json:"customer_id"      binding:"required,uuid"`
}

func (h *SharedCartHandler) CreateCart(c *gin.Context) {
	var req CreateCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tableSessionID, _ := uuid.Parse(req.TableSessionID)
	customerID, _ := uuid.Parse(req.CustomerID)
	cart, err := h.Service.CreateCart(c.Request.Context(), tableSessionID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

type JoinCartRequest struct {
	CartID     string `json:"cart_id"     binding:"required,uuid"`
	CustomerID string `json:"customer_id" binding:"required,uuid"`
}

func (h *SharedCartHandler) JoinCart(c *gin.Context) {
	var req JoinCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cartID, _ := uuid.Parse(req.CartID)
	customerID, _ := uuid.Parse(req.CustomerID)
	if err := h.Service.JoinCart(c.Request.Context(), cartID, customerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "joined"})
}

type AddItemRequest struct {
	CartID     string `json:"cart_id"      binding:"required,uuid"`
	MenuItemID string `json:"menu_item_id" binding:"required,uuid"`
	Quantity   int32  `json:"quantity"     binding:"required,min=1,max=99"`
	CustomerID string `json:"customer_id"  binding:"required,uuid"`
}

func (h *SharedCartHandler) AddItem(c *gin.Context) {
	var req AddItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cartID, _ := uuid.Parse(req.CartID)
	menuID, _ := uuid.Parse(req.MenuItemID)
	customerID, _ := uuid.Parse(req.CustomerID)
	item, err := h.Service.AddItem(c.Request.Context(), cartID, menuID, req.Quantity, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}
