package handler

import (
	"net/http"
	"os"
	"strings"

	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	Hub *notifications.Hub
}

func NewWSHandler(hub *notifications.Hub) *WSHandler {
	return &WSHandler{Hub: hub}
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

func (h *WSHandler) HandleWS(c *gin.Context) {
	branchID := c.Query("branch_id")
	if branchID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch_id is required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := notifications.NewClient(conn, branchID)

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
	// When the read fails (disconnect / error), we unregister synchronously so
	// the Hub closes client.send, which in turn causes WritePump to return.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.Hub.Unregister(client)
			conn.Close()
			break
		}
	}
}
