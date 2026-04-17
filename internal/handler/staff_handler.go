package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type StaffHandler struct {
	Service      *service.AuthService
	StaffService *service.StaffService
}

func NewStaffHandler(auth *service.AuthService, staff *service.StaffService) *StaffHandler {
	return &StaffHandler{
		Service:      auth,
		StaffService: staff,
	}
}

var validRoles = map[string]bool{
	"manager": true,
	"waiter":  true,
	"kitchen": true,
	"cashier": true,
}

type CreateStaffRequest struct {
	Name     string `json:"name"      binding:"required,min=2,max=100"`
	Email    string `json:"email"     binding:"required,email"`
	Password string `json:"password"  binding:"required,min=6"`
	Role     string `json:"role"      binding:"required"`
	BranchID string `json:"branch_id" binding:"required,uuid"`
}

func (h *StaffHandler) CreateStaff(c *gin.Context) {
	var req CreateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be one of: manager, waiter, kitchen, cashier"})
		return
	}

	tenantIDStr, _ := c.Get("tenant_id")
	userIDStr, _ := c.Get("user_id")

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id in token"})
		return
	}

	createdBy, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id in token"})
		return
	}

	branchID, err := uuid.Parse(req.BranchID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	staff, err := h.Service.RegisterStaff(
		c.Request.Context(),
		tenantID,
		branchID,
		createdBy,
		req.Name,
		req.Email,
		req.Password,
		req.Role,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, staff)
}

func (h *StaffHandler) GetStaff(c *gin.Context) {
	branchIDRaw := c.Query("branch_id")
	if branchIDRaw == "" {
		branchIDVal, _ := c.Get("branch_id")
		branchIDRaw, _ = branchIDVal.(string)
	}

	branchID, err := uuid.Parse(branchIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	staff, err := h.StaffService.GetStaffByBranch(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, staff)
}

func (h *StaffHandler) DeleteStaff(c *gin.Context) {
	staffID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid staff id"})
		return
	}

	if err := h.StaffService.DeleteStaff(c.Request.Context(), staffID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "staff deleted"})
}
