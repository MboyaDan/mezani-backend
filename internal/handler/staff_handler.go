package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type StaffHandler struct {
	Service      *service.AuthService
	StaffService *service.StaffService
	// GetTenantSubscription is injected (not a direct DB dependency) so
	// /me can return authoritative subscription status alongside profile
	// info — the frontend's trial/expiry UI needs this to be real server
	// state, not a client-side JWT calculation that goes stale the
	// moment a tenant actually renews.
	GetTenantSubscription func(ctx context.Context, tenantID uuid.UUID) (status string, expiresAt time.Time, err error)
}

func NewStaffHandler(
	auth *service.AuthService,
	staff *service.StaffService,
	getTenantSubscription func(ctx context.Context, tenantID uuid.UUID) (string, time.Time, error),
) *StaffHandler {
	return &StaffHandler{
		Service:               auth,
		StaffService:          staff,
		GetTenantSubscription: getTenantSubscription,
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

type MeResponse struct {
	ID                    string `json:"id"`
	TenantID              string `json:"tenant_id"`
	BranchID              string `json:"branch_id,omitempty"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Role                  string `json:"role"`
	SubscriptionStatus    string `json:"subscription_status"`
	SubscriptionExpiresAt string `json:"subscription_expires_at"`
}

func (h *StaffHandler) Me(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
		return
	}

	staff, err := h.StaffService.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "staff not found"})
			return
		}
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to fetch staff"})
		return
	}

	subStatus, subExpiresAt, err := h.GetTenantSubscription(c.Request.Context(), staff.TenantID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to fetch subscription status"})
		return
	}

	resp := MeResponse{
		ID:                    staff.ID.String(),
		TenantID:              staff.TenantID.String(),
		Name:                  staff.Name,
		Email:                 staff.Email,
		Role:                  staff.Role,
		SubscriptionStatus:    subStatus,
		SubscriptionExpiresAt: subExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if staff.BranchID.Valid {
		resp.BranchID = uuid.UUID(staff.BranchID.Bytes).String()
	}

	c.JSON(http.StatusOK, resp)
}
