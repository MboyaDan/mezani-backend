package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
)

type ContactHandler struct {
	EmailSender *notifications.EmailSender
	Recipients  []string
	Logger      *slog.Logger
}

func NewContactHandler(sender *notifications.EmailSender, recipients []string, logger *slog.Logger) *ContactHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContactHandler{EmailSender: sender, Recipients: recipients, Logger: logger}
}

type ContactRequest struct {
	Name    string `json:"name" binding:"required,min=2,max=100"`
	Email   string `json:"email" binding:"required,email"`
	Message string `json:"message" binding:"required,min=10,max=2000"`
	// Website is a honeypot field — hidden from real users via CSS on the
	// frontend, but bots that blindly fill every input on a form tend to
	// fill it too. A real submission always leaves it empty.
	Website string `json:"website"`
}

func (h *ContactHandler) Submit(c *gin.Context) {
	var req ContactRequest

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&req); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body is too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Website != "" {
		c.JSON(http.StatusOK, gin.H{"message": "Thanks — we'll be in touch soon."})
		return
	}

	if len(h.Recipients) == 0 {
		h.Logger.ErrorContext(c.Request.Context(), "contact form submitted but no recipient emails configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send your message — please try again later"})
		return
	}

	if err := h.EmailSender.SendContactMessage(c.Request.Context(), h.Recipients, notifications.ContactMessage{
		Name:    req.Name,
		Email:   req.Email,
		Message: req.Message,
	}); err != nil {
		h.Logger.ErrorContext(c.Request.Context(), "failed to send contact form email", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send your message — please try again later"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Thanks — we'll be in touch soon."})
}
