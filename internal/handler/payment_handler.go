package handler

import (
	"errors"
	"net/http"

	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	Service *service.PaymentService
}

func NewPaymentHandler(s *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{Service: s}
}

type InitiateCashPaymentRequest struct {
	TableSessionID string `json:"table_session_id" binding:"required,uuid"`
}

// InitiateCash — called when a customer/waiter selects "pay cash".
// Creates a pending payment for the FULL outstanding balance of the table
// session, computed server-side. The amount is never accepted from the
// client — see PaymentService.InitiateCashPayment for why.
func (h *PaymentHandler) InitiateCash(c *gin.Context) {
	var req InitiateCashPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tableSessionID, err := uuid.Parse(req.TableSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table_session_id"})
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

	var initiatedBy *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			initiatedBy = &id
		}
	}

	payment, err := h.Service.InitiateCashPayment(
		c.Request.Context(), tableSessionID, tenantID, initiatedBy,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPaymentAlreadyPending):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrUnauthorizedTableSession):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrTableSessionNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrNothingToPay):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, payment)
}

type ConfirmPaymentRequest struct {
	PaymentID string `json:"payment_id" binding:"required,uuid"`
}

// Confirm — cashier confirms cash was physically received. Marks the
// payment confirmed and closes the bill (orders -> paid, session closed).
func (h *PaymentHandler) Confirm(c *gin.Context) {
	var req ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	paymentID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment_id"})
		return
	}

	payment, err := h.Service.ConfirmCashPayment(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payment)
}

// Pending — cashier dashboard's "awaiting confirmation" queue.
// Tenant ownership of :branch_id is verified upstream by
// middleware.TenantBranchGuard (see router.go) before this ever runs.
func (h *PaymentHandler) Pending(c *gin.Context) {
	branchIDStr := c.Param("branch_id")
	branchID, err := uuid.Parse(branchIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}

	payments, err := h.Service.GetPendingCashPayments(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}
