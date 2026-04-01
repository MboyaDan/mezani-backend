package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type MenuHandler struct {
	Service *service.MenuService
}

func NewMenuHandler(s *service.MenuService) *MenuHandler {
	return &MenuHandler{Service: s}
}

//
// REQUEST STRUCTS
//

type CreateMenuRequest struct {
	BranchID string `json:"branch_id" binding:"required"` // Changed from RestaurantID
}

type CreateCategoryRequest struct {
	MenuID string `json:"menu_id" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Order  int32  `json:"order" binding:"required"`
}

type CreateMenuItemRequest struct {
	CategoryID  string  `json:"category_id" binding:"required"` // Changed from MenuID
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required"`
}

type UpdatePriceRequest struct {
	Price float64 `json:"price" binding:"required"`
}

//
// RESPONSE TYPES FOR FULL MENU
//

type MenuItemResponse struct {
	ID          uuid.UUID `json:"ID"`
	Name        string    `json:"Name"`
	Description string    `json:"Description"`
	Price       float64   `json:"Price"`
	Available   bool      `json:"Available"`
	SoldOut     bool      `json:"SoldOut"`
	IsSpecial   bool      `json:"IsSpecial"`
}

type CategoryResponse struct {
	CategoryID   uuid.UUID          `json:"category_id"`
	CategoryName string             `json:"category_name"`
	DisplayOrder int32              `json:"display_order"`
	Items        []MenuItemResponse `json:"items"`
}

//
// CREATE MENU
//

func (h *MenuHandler) CreateMenu(c *gin.Context) {
	var req CreateMenuRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	branchID, err := uuid.Parse(req.BranchID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	menu, err := h.Service.CreateMenu(
		c.Request.Context(),
		branchID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, menu)
}

//
// CREATE CATEGORY
//

func (h *MenuHandler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	menuID, err := uuid.Parse(req.MenuID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid menu_id"})
		return
	}

	category, err := h.Service.CreateCategory(
		c.Request.Context(),
		menuID,
		req.Name,
		req.Order,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, category)
}

//
// CREATE MENU ITEM
//

func (h *MenuHandler) CreateMenuItem(c *gin.Context) {
	var req CreateMenuItemRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
		return
	}

	item, err := h.Service.AddMenuItem(
		c.Request.Context(),
		categoryID,
		req.Name,
		req.Description,
		req.Price,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

//
// GET MENUS BY BRANCH
//

func (h *MenuHandler) GetBranchMenus(c *gin.Context) {
	branchIDStr := c.Param("branch_id")

	branchID, err := uuid.Parse(branchIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	menus, err := h.Service.GetBranchMenus(
		c.Request.Context(),
		branchID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, menus)
}

//
// GET MENU CATEGORIES
//

func (h *MenuHandler) GetMenuCategories(c *gin.Context) {
	menuIDStr := c.Param("menu_id")

	menuID, err := uuid.Parse(menuIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid menu_id"})
		return
	}

	categories, err := h.Service.GetMenuCategories(
		c.Request.Context(),
		menuID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, categories)
}

//
// GET CATEGORY ITEMS
//

func (h *MenuHandler) GetCategoryItems(c *gin.Context) {
	categoryIDStr := c.Param("category_id")

	categoryID, err := uuid.Parse(categoryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
		return
	}

	items, err := h.Service.GetCategoryItems(
		c.Request.Context(),
		categoryID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

//
// GET FULL MENU (NESTED JSON)
//

func (h *MenuHandler) GetFullMenu(c *gin.Context) {
	menuIDStr := c.Param("menu_id")

	menuID, err := uuid.Parse(menuIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid menu_id"})
		return
	}

	rows, err := h.Service.GetFullMenu(c.Request.Context(), menuID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Transform into nested JSON response
	response := make([]CategoryResponse, 0, len(rows))

	for _, row := range rows {
		var items []MenuItemResponse
		if err := json.Unmarshal(row.Items, &items); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse menu items"})
			return
		}

		category := CategoryResponse{
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			DisplayOrder: row.DisplayOrder,
			Items:        items,
		}

		response = append(response, category)
	}

	c.JSON(http.StatusOK, response)
}

//
// UPDATE MENU ITEM PRICE
//

func (h *MenuHandler) UpdatePrice(c *gin.Context) {
	idStr := c.Param("id")
	var req UpdatePriceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	err = h.Service.UpdateMenuItemPrice(
		c.Request.Context(),
		id,
		req.Price,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "price updated"})
}

//
// SET ITEM SOLD OUT
//

func (h *MenuHandler) SetItemSoldOut(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	// You need to get the current state first or modify service
	// For now, setting to true
	err = h.Service.SetMenuItemSoldOut(
		c.Request.Context(),
		id,
		true,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item marked sold out"})
}

//
// SET ITEM AVAILABLE
//

func (h *MenuHandler) SetItemAvailable(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	err = h.Service.SetMenuItemAvailability(
		c.Request.Context(),
		id,
		true,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item marked available"})
}

//
// SET DAILY SPECIAL
//

func (h *MenuHandler) SetDailySpecial(c *gin.Context) {
	idStr := c.Param("id")
	isSpecialStr := c.Query("special") // ?special=true

	isSpecial := isSpecialStr == "true"

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	err = h.Service.SetDailySpecial(
		c.Request.Context(),
		id,
		isSpecial,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "daily special updated"})
}

func (h *MenuHandler) GetMenuBySession(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	rows, err := h.Service.GetMenuBySession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Same transform as GetFullMenu — deserialize Items []byte → []MenuItemResponse
	response := make([]CategoryResponse, 0, len(rows))
	for _, row := range rows {
		var items []MenuItemResponse
		if err := json.Unmarshal(row.Items, &items); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse menu items"})
			return
		}
		response = append(response, CategoryResponse{
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			DisplayOrder: row.DisplayOrder,
			Items:        items,
		})
	}

	c.JSON(http.StatusOK, response)
}
