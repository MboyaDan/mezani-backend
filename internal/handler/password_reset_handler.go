package handler

import (
	"mezzani_backend/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PasswordResetHandler struct {
	Service *service.PasswordResetService
}

func NewPasswordResetHandler(s *service.PasswordResetService) *PasswordResetHandler {
	return &PasswordResetHandler{Service: s}
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *PasswordResetHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Always return 200 — don't reveal if email exists
	h.Service.RequestReset(c.Request.Context(), req.Email)
	c.JSON(http.StatusOK, gin.H{"message": "If that email exists you will receive a reset link shortly"})
}

func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}
