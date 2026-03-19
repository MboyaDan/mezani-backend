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

type CreateStaffRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	BranchID string `json:"branch_id"`
}

func (h *StaffHandler) CreateStaff(c *gin.Context) {

	var req CreateStaffRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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
