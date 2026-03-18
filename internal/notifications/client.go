package notifications

import "github.com/gorilla/websocket"

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		conn: conn,
		send: make(chan []byte),
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
