package handler

import (
	"net/http"
	"strings"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AIHandler struct {
	Service *service.AIService
}

func NewAIHandler(s *service.AIService) *AIHandler {
	return &AIHandler{Service: s}
}

type AIChatRequest struct {
	Message   string `json:"message"    binding:"required,min=2,max=500"`
	SessionID string `json:"session_id"`
}

func (h *AIHandler) Chat(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use comma-ok on every c.Get to avoid panics on nil interface assertions.
	tenantIDRaw, _ := c.Get("tenant_id")
	userIDRaw, _ := c.Get("user_id")
	branchIDRaw, _ := c.Get("branch_id")
	tenantNameRaw, _ := c.Get("tenant_name")

	tenantStr, ok := tenantIDRaw.(string)
	if !ok || tenantStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant claim"})
		return
	}
	tenantID, err := uuid.Parse(tenantStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant"})
		return
	}

	// userID is best-effort — zero UUID is acceptable for logging purposes.
	userStr, _ := userIDRaw.(string)
	userID, _ := uuid.Parse(userStr)

	branchIDStr, _ := branchIDRaw.(string)
	if branchIDStr == "" || branchIDStr == "00000000-0000-0000-0000-000000000000" {
		branchIDStr = c.Query("branch_id")
	}
	branchID, err := uuid.Parse(branchIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch_id required — include as query param or ensure it is in your token"})
		return
	}

	restaurantName, _ := tenantNameRaw.(string)

	resp, err := h.Service.Chat(c.Request.Context(), service.AIChatRequest{
		TenantID:       tenantID,
		BranchID:       branchID,
		UserID:         userID,
		RestaurantName: restaurantName,
		Message:        req.Message,
		SessionID:      req.SessionID,
	})
	if err != nil {
		status := http.StatusServiceUnavailable
		if strings.Contains(err.Error(), "rate limit") {
			status = http.StatusTooManyRequests
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
