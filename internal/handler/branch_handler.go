package handler

import (
	"errors"
	"net/http"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BranchHandler struct {
	Service *service.BranchService
}

func NewBranchHandler(s *service.BranchService) *BranchHandler {
	return &BranchHandler{Service: s}
}

type CreateBranchRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func (h *BranchHandler) CreateBranch(c *gin.Context) {
	var req CreateBranchRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid tenant_id in token"})
		return
	}

	branch, err := h.Service.CreateBranch(
		c.Request.Context(),
		tenantID,
		req.Name,
		req.Location,
	)

	if err != nil {
		if errors.Is(err, service.ErrBranchLimitReached) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You've reached the branch limit for your current plan. Upgrade to add more branches.",
				"code":  "branch_limit_reached",
			})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, branch)
}

func (h *BranchHandler) ListBranches(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid tenant_id"})
		return
	}
	branches, err := h.Service.GetBranchesByTenant(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, branches)
}

func (h *BranchHandler) DeleteBranch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Service.DeleteBranch(c.Request.Context(), id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "branch deleted"})
}
