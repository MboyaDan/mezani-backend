package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mezzani_backend/internal/common"
	"mezzani_backend/internal/service"
)

type AuthHandler struct {
	Service *service.AuthService
}

func NewAuthHandler(s *service.AuthService) *AuthHandler {
	return &AuthHandler{Service: s}
}

// ---------------- REQUEST TYPES ----------------

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterOwnerRequest struct {
	RestaurantName string `json:"restaurant_name" binding:"required,min=2"`
	Email          string `json:"email"           binding:"required,email"`
	Password       string `json:"password"        binding:"required,min=8"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ---------------- ERROR MAPPING ----------------

// httpError maps typed service/common errors to an appropriate HTTP status code
// and a safe, non-leaking message. Internal errors and unrecognised errors both
// return 500 with a generic message — the real cause is logged at the service
// layer, never exposed to the caller.
func httpError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})

	case errors.Is(err, common.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})

	case errors.Is(err, common.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})

	case errors.Is(err, common.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})

	default:
		// ErrInternalServer and anything unexpected both return the same safe
		// message. The service layer already logged the real cause.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
	}
}

// ---------------- HANDLERS ----------------

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	pair, err := h.Service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AuthHandler) RegisterOwner(c *gin.Context) {
	var req RegisterOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restaurant_name, email, and password are required"})
		return
	}

	pair, err := h.Service.RegisterOwner(
		c.Request.Context(),
		req.RestaurantName,
		req.Email,
		req.Password,
	)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusCreated, pair)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	pair, err := h.Service.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	if err := h.Service.RevokeRefreshToken(c.Request.Context(), req.RefreshToken); err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
