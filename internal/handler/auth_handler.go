package handler

import (
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Service *service.AuthService
}

func NewAuthHandler(s *service.AuthService) *AuthHandler {
	return &AuthHandler{Service: s}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {

	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, err)
		return
	}

	token, err := h.Service.Login(
		c.Request.Context(),
		req.Email,
		req.Password,
	)

	if err != nil {
		c.JSON(401, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"token": token,
	})
}
