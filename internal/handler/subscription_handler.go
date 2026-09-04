package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SubscriptionHandler struct {
	Service  *service.SubscriptionService
	Paystack *service.PaystackClient
	Logger   *slog.Logger
}

func NewSubscriptionHandler(s *service.SubscriptionService, paystack *service.PaystackClient, logger *slog.Logger) *SubscriptionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SubscriptionHandler{Service: s, Paystack: paystack, Logger: logger}
}

func (h *SubscriptionHandler) Plans(c *gin.Context) {
	plans, err := h.Service.ListPlans(c.Request.Context())
	if err != nil {
		h.Logger.ErrorContext(c.Request.Context(), "failed to list plans", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load plans"})
		return
	}

	type planResponse struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		DisplayName       string  `json:"display_name"`
		PriceKes          float64 `json:"price_kes"`
		BillingPeriodDays int32   `json:"billing_period_days"`
		MaxBranches       int32   `json:"max_branches"`
	}

	result := make([]planResponse, 0, len(plans))
	for _, p := range plans {
		result = append(result, planResponse{
			ID:                p.ID.String(),
			Name:              p.Name,
			DisplayName:       p.DisplayName,
			PriceKes:          p.PriceKes,
			BillingPeriodDays: p.BillingPeriodDays,
			MaxBranches:       p.MaxBranches,
		})
	}

	c.JSON(http.StatusOK, result)
}

type InitiateRenewalRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
}

func (h *SubscriptionHandler) InitiateRenewal(c *gin.Context) {
	var req InitiateRenewalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant"})
		return
	}

	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
		return
	}
	staffID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	authURL, err := h.Service.InitiateRenewal(c.Request.Context(), tenantID, req.PlanName, staffID)
	if err != nil {
		if errors.Is(err, service.ErrPlanNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown plan"})
			return
		}
		h.Logger.ErrorContext(c.Request.Context(), "failed to initiate renewal", "error", err, "tenant_id", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start checkout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"authorization_url": authURL})
}

type VerifyRequest struct {
	Reference string `json:"reference" binding:"required"`
}

// Verify — the authenticated redirect-fallback path. tenant_id is pulled
// from the caller's own JWT and enforced against the payment record
// inside VerifyAndActivateForTenant, so one tenant can never activate or
// read back another tenant's subscription by guessing/submitting their
// payment reference.
func (h *SubscriptionHandler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant"})
		return
	}

	tenant, err := h.Service.VerifyAndActivateForTenant(c.Request.Context(), tenantID, req.Reference)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPaymentReferenceUnknown):
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown payment reference"})
		case errors.Is(err, service.ErrPaymentNotSuccessful):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment was not successful"})
		case errors.Is(err, service.ErrAmountMismatch):
			h.Logger.ErrorContext(c.Request.Context(), "subscription payment amount mismatch", "reference", req.Reference)
			c.JSON(http.StatusConflict, gin.H{"error": "payment verification failed"})
		case errors.Is(err, service.ErrPlanUnavailable):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Your payment was received, but we couldn't activate this plan. Please contact support with this reference: " + req.Reference,
			})
		default:
			h.Logger.ErrorContext(c.Request.Context(), "verify failed", "error", err, "reference", req.Reference)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "verification failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_status":     tenant.SubscriptionStatus,
		"subscription_expires_at": tenant.SubscriptionExpiresAt,
	})
}

const maxWebhookBodyBytes = 1 << 20 // 1MB — Paystack payloads are small; bound the read since this endpoint is unauthenticated until the signature check below

type paystackWebhookEvent struct {
	Event string `json:"event"`
	Data  struct {
		Reference string `json:"reference"`
	} `json:"data"`
}

// Webhook — Paystack calling us server-to-server, with no staff JWT and
// no tenant scope at all. It still calls the plain VerifyAndActivate
// (not the *ForTenant variant) — its authority is the verified HMAC
// signature above, not a tenant-ownership check, which wouldn't make
// sense here since there's no caller tenant to check against.
func (h *SubscriptionHandler) Webhook(c *gin.Context) {
	rawBody, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	signature := c.GetHeader("x-paystack-signature")
	if signature == "" || !h.Paystack.VerifyWebhookSignature(rawBody, signature) {
		h.Logger.WarnContext(c.Request.Context(), "rejected paystack webhook: invalid signature")
		c.Status(http.StatusUnauthorized)
		return
	}

	// Unmarshal the bytes already read above — c.Request.Body has already
	// been drained by io.ReadAll and is NOT restored, so calling
	// c.ShouldBindJSON here (which reads from that same, now-empty body)
	// would always fail with io.EOF. This was silently breaking every
	// webhook delivery: the handler always returned 400 before
	// VerifyAndActivate ever ran.
	var event paystackWebhookEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if event.Event == "charge.success" && event.Data.Reference != "" {
		if _, err := h.Service.VerifyAndActivate(c.Request.Context(), event.Data.Reference); err != nil {
			h.Logger.ErrorContext(c.Request.Context(), "webhook-triggered verify failed", "error", err, "reference", event.Data.Reference)
		}
	}

	c.Status(http.StatusOK)
}
