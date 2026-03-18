package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/service"
)

type TableSessionHandler struct {
	Service *service.TableSessionService
}

func NewTableSessionHandler(s *service.TableSessionService) *TableSessionHandler {
	return &TableSessionHandler{Service: s}
}

type StartSessionRequest struct {
	TableID  string `json:"table_id"`
	Duration int    `json:"duration_minutes"`
}

func (h *TableSessionHandler) StartSession(c *gin.Context) {

	var req StartSessionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tableID, err := uuid.Parse(req.TableID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table id"})
		return
	}

	session, err := h.Service.StartSession(c.Request.Context(), tableID, req.Duration)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *TableSessionHandler) CloseSession(c *gin.Context) {

	id := c.Param("id")

	sessionID, err := uuid.Parse(id)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.Service.CloseSession(c.Request.Context(), sessionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "session closed"})
}

type HeartbeatRequest struct {
	SessionID string `json:"session_id"`
}

func (h *TableSessionHandler) Heartbeat(c *gin.Context) {

	var req HeartbeatRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, err)
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)

	if err != nil {
		c.JSON(400, "invalid session id")
		return
	}

	err = h.Service.Heartbeat(
		c.Request.Context(),
		sessionID,
	)

	if err != nil {
		c.JSON(500, err)
		return
	}

	c.JSON(200, gin.H{
		"status": "extended",
	})
}
