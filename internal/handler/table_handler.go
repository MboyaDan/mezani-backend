package handler

import (
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TableHandler struct {
	Service *service.TableService
}

func NewTableHandler(s *service.TableService) *TableHandler {
	return &TableHandler{Service: s}
}

type CreateTableRequest struct {
	TableNumber int32 `json:"table_number"`
}

func (h *TableHandler) CreateTable(c *gin.Context) {
	var req CreateTableRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	branchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid branch_id"})
		return
	}

	table, err := h.Service.CreateTable(
		c.Request.Context(),
		branchID,
		req.TableNumber,
	)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, table)
}
