package handler

import (
	"net/http"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InventoryHandler struct {
	Service *service.InventoryService
}

func NewInventoryHandler(s *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{Service: s}
}

// ============================================================
// REQUEST TYPES
// ============================================================

type CreateInventoryItemRequest struct {
	Name      string `json:"name"      binding:"required"`
	Stock     int32  `json:"stock"     binding:"min=0"`
	Threshold int32  `json:"threshold" binding:"min=0"`
}

type SetStockRequest struct {
	Stock int32 `json:"stock" binding:"required,min=0"`
}

// ============================================================
// HANDLERS
// ============================================================

// POST /branches/:branch_id/inventory
func (h *InventoryHandler) CreateItem(c *gin.Context) {
	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	var req CreateInventoryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.Service.CreateItem(
		c.Request.Context(),
		branchID,
		req.Name,
		req.Stock,
		req.Threshold,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create inventory item"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// GET /branches/:branch_id/inventory
func (h *InventoryHandler) ListItems(c *gin.Context) {
	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	items, err := h.Service.GetByBranch(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch inventory"})
		return
	}

	c.JSON(http.StatusOK, items)
}

// GET /branches/:branch_id/inventory/:item_id
func (h *InventoryHandler) GetItem(c *gin.Context) {
	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	item, err := h.Service.GetByID(c.Request.Context(), itemID, branchID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "inventory item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

// PATCH /branches/:branch_id/inventory/:item_id/stock
func (h *InventoryHandler) SetStock(c *gin.Context) {
	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	var req SetStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.Service.SetStock(c.Request.Context(), itemID, branchID, req.Stock)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update stock"})
		return
	}

	c.JSON(http.StatusOK, item)
}

// GET /branches/:branch_id/inventory/low-stock
func (h *InventoryHandler) LowStockAlerts(c *gin.Context) {
	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	items, err := h.Service.GetLowStock(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch low stock items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(items),
		"items": items,
	})
}
