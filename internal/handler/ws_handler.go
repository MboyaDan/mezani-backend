package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	db "mezzani_backend/internal/database/sqlc"
	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
)

// BranchLookup is the one thing the WebSocket handler needs from the branch
// service: who owns a branch.
type BranchLookup interface {
	GetBranchByID(ctx context.Context, branchID uuid.UUID) (db.Branch, error)
}

type WSHandler struct {
	Hub      *notifications.Hub
	Tickets  *notifications.WSTicketService
	Branches BranchLookup
}

func NewWSHandler(hub *notifications.Hub, jwtSecret []byte, branches BranchLookup) *WSHandler {
	return &WSHandler{
		Hub:      hub,
		Tickets:  notifications.NewWSTicketService(jwtSecret),
		Branches: branches,
	}
}

// allowedOrigins returns the set of permitted origins from the environment.
// Set ALLOWED_ORIGINS as a comma-separated list, e.g.:
//
//	ALLOWED_ORIGINS=https://app.mezzani.co,https://admin.mezzani.co
//
// Falls back to blocking all cross-origin requests when the variable is unset,
// which is the safest default.
func allowedOrigins() map[string]struct{} {
	raw := os.Getenv("ALLOWED_ORIGINS")
	set := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return set
}

var upgrader = websocket.Upgrader{
	// FIX 1 (CRITICAL): Validate the request Origin against an allowlist instead of
	// accepting every origin. Accepting all origins lets any website open a
	// WebSocket to your server on behalf of a logged-in user (CSRF over WS).
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Same-origin requests (e.g. server-side clients) have no Origin header.
		if origin == "" {
			return true
		}
		_, ok := allowedOrigins()[origin]
		return ok
	},
}

// IssueTicket (POST /api/ws/ticket, behind AuthMiddleware) exchanges a valid access
// token for a short-lived, single-use WebSocket ticket for one branch.
//
// The caller must be signed-in staff, the branch must belong to their tenant, and
// branch-pinned staff (waiter, kitchen, ...) may only ask for their own branch.
// Every refusal looks the same, so the endpoint cannot be used to discover which
// branch IDs exist.
func (h *WSHandler) IssueTicket(c *gin.Context) {
	var req struct {
		BranchID string `json:"branch_id" binding:"required,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch_id is required"})
		return
	}
	branchID, _ := uuid.Parse(req.BranchID) // already validated by the binding tag

	role := c.GetString("role")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	tokenBranchID := c.GetString("branch_id")

	// Only "no such branch" is an authorization answer. Any other failure (timeout,
	// connection loss, ...) is OUR problem: answering 403 would tell valid staff they
	// are not allowed and would log an outage as an access denial.
	branchTenantID := ""
	branch, err := h.Branches.GetBranchByID(c.Request.Context(), branchID)
	switch {
	case err == nil:
		branchTenantID = branch.TenantID.String()
	case errors.Is(err, pgx.ErrNoRows):
		// Missing branch: falls through to the same uniform 403 as any other refusal.
	default:
		log.Printf("ws ticket: branch lookup failed | user=%s branch=%s err=%v", userID, branchID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not verify branch access, please try again"})
		return
	}

	if err := notifications.AuthorizeBranchSubscription(
		role, tokenBranchID, tenantID, branchID.String(), branchTenantID,
	); err != nil {
		if errors.Is(err, notifications.ErrWSForbiddenRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "your role cannot use live updates"})
			return
		}
		log.Printf("ws ticket denied | user=%s role=%s branch=%s", userID, role, branchID)
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed to subscribe to this branch"})
		return
	}

	ticket, ttl, err := h.Tickets.Issue(userID, tenantID, branchID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue ticket"})
		return
	}

	// A ticket is a credential: never let an intermediary cache it.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expires_in": int(ttl.Seconds())})
}

// HandleWS (GET /ws/kitchen?ticket=...) upgrades to a WebSocket for the branch the
// ticket was issued for. The branch comes from the signed ticket, never from the
// query string, so a client cannot change it.
func (h *WSHandler) HandleWS(c *gin.Context) {
	ticket := c.Query("ticket")
	if ticket == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ticket is required"})
		return
	}

	claims, err := h.Tickets.Redeem(ticket)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired ticket"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// This socket is receive-only. Bound what a client can make us read, and drop
	// connections that stop answering pings.
	conn.SetReadLimit(notifications.MaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(notifications.PongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(notifications.PongWait))
	})

	client := notifications.NewClient(conn, claims.BranchID)

	// FIX 2 (Race condition): Register the client BEFORE starting WritePump.
	// Previously, the goroutine could attempt to write to client.send before
	// the Hub had added the client to its map, and the Hub could close
	// client.send (on unregister) before WritePump had even started —
	// causing a send-on-closed-channel panic.
	h.Hub.Register(client)

	// WritePump runs in its own goroutine and owns all writes to the connection.
	// It exits when client.send is closed by the Hub (unregister path).
	go client.WritePump()

	// Read loop — keeps the connection alive and detects client disconnects.
	// When the read fails (disconnect / error / missed pong), we unregister
	// synchronously so the Hub closes client.send, which in turn causes
	// WritePump to return.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.Hub.Unregister(client)
			conn.Close()
			break
		}
	}
}
