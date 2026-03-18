package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/service"
)

type CustomerSessionHandler struct {
	Service *service.CustomerSessionService
}

func NewCustomerSessionHandler(s *service.CustomerSessionService) *CustomerSessionHandler {
	return &CustomerSessionHandler{Service: s}
}

type JoinTableRequest struct {
	TableSessionID string `json:"table_session_id"`
	Name           string `json:"name"`
}

// JoinTable handles POST /customer/join
func (h *CustomerSessionHandler) JoinTable(c *gin.Context) {
	var req JoinTableRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID, err := uuid.Parse(req.TableSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table session id"})
		return
	}

	customer, err := h.Service.JoinTableSession(
		c.Request.Context(),
		sessionID,
		req.Name,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// GetCustomer handles GET /customer/:id
func (h *CustomerSessionHandler) GetCustomer(c *gin.Context) {
	id := c.Param("id")

	customerID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	customer, err := h.Service.GetCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// ListCustomers handles GET /customers?table_session_id=xxx
// OR GET /table-session/:id/customers
func (h *CustomerSessionHandler) ListCustomers(c *gin.Context) {
	// Try to get from query param first
	id := c.Query("table_session_id")

	// If not in query, try path param
	if id == "" {
		id = c.Param("table_session_id")
	}

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "table_session_id is required"})
		return
	}

	sessionID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table session id"})
		return
	}

	customers, err := h.Service.ListCustomers(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return empty array instead of null if no customers
	if customers == nil {
		customers = []db.CustomerSession{}
	}

	c.JSON(http.StatusOK, customers)
}
