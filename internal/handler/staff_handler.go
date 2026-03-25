package handler

import (
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StaffHandler struct {
	Service *service.AuthService
}

func NewStaffHandler(s *service.AuthService) *StaffHandler {
	return &StaffHandler{Service: s}
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
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if !validRoles[req.Role] {
		c.JSON(400, gin.H{"error": "invalid role, must be one of: manager, waiter, kitchen, cashier"})
		return
	}

	tenantIDStr, _ := c.Get("tenant_id")
	userIDStr, _ := c.Get("user_id")

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid tenant_id in token"})
		return
	}

	createdBy, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid user_id in token"})
		return
	}

	branchID, err := uuid.Parse(req.BranchID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid branch_id"})
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, staff)
}
