package handler

import (
	"net/http"

	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	Hub *notifications.Hub
}

func NewWSHandler(hub *notifications.Hub) *WSHandler {
	return &WSHandler{
		Hub: hub,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *WSHandler) HandleWS(c *gin.Context) {

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "websocket upgrade failed",
		})
		return
	}

	client := notifications.NewClient(conn)

	h.Hub.Register(client)

	go client.WritePump()
}
