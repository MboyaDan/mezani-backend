package notifications

import "github.com/gorilla/websocket"

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

func (c *Client) WritePump() {
	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			c.conn.Close()
			return
		}
	}
}
