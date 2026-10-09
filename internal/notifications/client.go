package notifications

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is how long a single write may take before the connection is dropped.
	writeWait = 10 * time.Second

	// PongWait is how long we wait for any sign of life (a pong) before treating the
	// client as gone. Browsers answer pings automatically.
	PongWait = 60 * time.Second

	// pingPeriod must be shorter than PongWait. It also keeps idle connections open
	// through proxies and load balancers that close silent sockets.
	pingPeriod = (PongWait * 9) / 10

	// MaxMessageSize caps what a client may send us. Clients never need to send
	// anything (the socket is receive-only), so this is deliberately tiny.
	MaxMessageSize = 512
)

type Client struct {
	BranchID string
	conn     *websocket.Conn
	send     chan []byte
}

func NewClient(conn *websocket.Conn, branchID string) *Client {
	return &Client{
		BranchID: branchID,
		conn:     conn,
		send:     make(chan []byte, 256), // buffered — prevents blocking the hub
	}
}

// WritePump owns every write to the connection. It exits when the hub closes
// client.send (unregister path) or when a write fails.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel: tell the client we are going away.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
