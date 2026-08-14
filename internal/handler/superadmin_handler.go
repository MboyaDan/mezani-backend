package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"mezzani_backend/internal/common"
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type SuperAdminHandler struct {
	Service *service.SuperAdminService
	Logger  *slog.Logger
}

func NewSuperAdminHandler(s *service.SuperAdminService, logger *slog.Logger) *SuperAdminHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SuperAdminHandler{Service: s, Logger: logger}
}

type SuperAdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *SuperAdminHandler) Login(c *gin.Context) {
	var req SuperAdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.Service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, common.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		h.Logger.ErrorContext(c.Request.Context(), "superadmin login failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

type SuperAdminRefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *SuperAdminHandler) Refresh(c *gin.Context) {
	var req SuperAdminRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.Service.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, common.ErrInvalidToken) || errors.Is(err, common.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			return
		}
		h.Logger.ErrorContext(c.Request.Context(), "superadmin refresh failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *SuperAdminHandler) Overview(c *gin.Context) {
	overview, err := h.Service.GetPlatformOverview(c.Request.Context())
	if err != nil {
		h.Logger.ErrorContext(c.Request.Context(), "failed to load platform overview", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load platform overview"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_count":           overview.TenantCount,
		"branch_count":           overview.BranchCount,
		"staff_count":            overview.StaffCount,
		"total_platform_revenue": overview.TotalPlatformRevenue,
	})
}

func (h *SuperAdminHandler) Tenants(c *gin.Context) {
	rows, err := h.Service.ListTenantsWithStats(c.Request.Context())
	if err != nil {
		h.Logger.ErrorContext(c.Request.Context(), "failed to list tenants", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tenants"})
		return
	}

	const trialDays = 14 * 24 * 60 * 60

	type tenantWithTrial struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		Plan          string  `json:"plan"`
		CreatedAt     string  `json:"created_at"`
		BranchCount   int32   `json:"branch_count"`
		StaffCount    int32   `json:"staff_count"`
		TotalRevenue  float64 `json:"total_revenue"`
		TrialExpired  bool    `json:"trial_expired"`
		TrialDaysLeft int     `json:"trial_days_left"`
	}

	result := make([]tenantWithTrial, 0, len(rows))
	for _, r := range rows {
		secondsLeft := (r.CreatedAt.Unix() + trialDays) - time.Now().Unix()
		daysLeft := int(secondsLeft / 86400)
		if daysLeft < 0 {
			daysLeft = 0
		}
		result = append(result, tenantWithTrial{
			ID:            r.ID.String(),
			Name:          r.Name,
			Plan:          r.Plan,
			CreatedAt:     r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			BranchCount:   r.BranchCount,
			StaffCount:    r.StaffCount,
			TotalRevenue:  r.TotalRevenue,
			TrialExpired:  secondsLeft <= 0,
			TrialDaysLeft: daysLeft,
		})
	}

	c.JSON(http.StatusOK, result)
}
