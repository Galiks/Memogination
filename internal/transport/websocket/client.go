package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Client is a single WebSocket connection bound to a room. It runs a read pump
// (discarding inbound messages) and a write pump (draining the send channel).
type Client struct {
	conn   *websocket.Conn
	roomID string
	send   chan []byte
	hub    *Hub

	// OnDisconnect is invoked once when the connection fully closes. It is
	// used to mark the associated player disconnected.
	OnDisconnect func()

	closeOnce sync.Once
}

// NewClient returns a Client for the given connection and room.
func NewClient(conn *websocket.Conn, roomID string, hub *Hub) *Client {
	return &Client{
		conn:   conn,
		roomID: roomID,
		send:   make(chan []byte, 64),
		hub:    hub,
	}
}

// Send queues a message for delivery. It is non-blocking; if the client's
// buffer is full the message is dropped.
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

// RoomID returns the room this client is bound to.
func (c *Client) RoomID() string { return c.roomID }

// Run starts the read and write pumps and blocks until the connection closes.
func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

// close unregisters the client, closes the send channel, and closes the
// connection exactly once. OnDisconnect fires only after the connection is
// fully closed (both pumps have exited).
func (c *Client) close() {
	c.closeOnce.Do(func() {
		c.hub.Unregister(c)
		close(c.send)
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
		if c.OnDisconnect != nil {
			c.OnDisconnect()
		}
	})
}

func (c *Client) writePump() {
	defer c.close()
	for msg := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			return
		}
	}
}

func (c *Client) readPump() {
	defer c.close()
	for {
		_, _, err := c.conn.Read(context.Background())
		if err != nil {
			return
		}
	}
}
