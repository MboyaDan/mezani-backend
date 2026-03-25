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
	TableID  string `json:"table_id"         binding:"required,uuid"`
	Duration int    `json:"duration_minutes" binding:"required,min=1,max=480"`
}

func (h *TableSessionHandler) StartSession(c *gin.Context) {
	var req StartSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tableID, _ := uuid.Parse(req.TableID)
	session, err := h.Service.StartSession(c.Request.Context(), tableID, req.Duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *TableSessionHandler) CloseSession(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}
	if err := h.Service.CloseSession(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "session closed"})
}

type HeartbeatRequest struct {
	SessionID string `json:"session_id" binding:"required,uuid"`
}

func (h *TableSessionHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessionID, _ := uuid.Parse(req.SessionID)
	if err := h.Service.Heartbeat(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "extended"})
}
