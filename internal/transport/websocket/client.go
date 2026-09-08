package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// SessionRevokedCloseCode is the WebSocket close code sent when a player's
// session is revoked (kicked or left): the client distinguishes it from a
// network drop and must not attempt to reconnect.
const SessionRevokedCloseCode = 4000

// Client is a single WebSocket connection bound to a room. It runs a read pump
// (discarding inbound messages) and a write pump (draining the send channel).
type Client struct {
	conn   *websocket.Conn
	roomID string
	// PlayerID is the authenticated player this connection belongs to, or "" for
	// admin/screen viewers. It lets the hub terminate a specific player's
	// connection when their session is revoked.
	PlayerID string
	send     chan []byte
	hub      *Hub

	// sendMu guards send/sendClosed so a Send racing a session-revoked close
	// can never write to a closed channel.
	sendMu     sync.Mutex
	sendClosed bool

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
// buffer is full the message is dropped. Safe to call concurrently with a
// session-revoked close (which closes the send channel).
func (c *Client) Send(data []byte) {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.sendClosed {
		return
	}
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
	c.closeWithCode(websocket.StatusNormalClosure, "")
}

// closeForSessionRevoked terminates the connection because the player's session
// was revoked (kick/leave), using a custom close code the client can recognize
// so it stops reconnecting.
func (c *Client) closeForSessionRevoked() {
	c.closeWithCode(websocket.StatusCode(SessionRevokedCloseCode), "session revoked")
}

func (c *Client) closeWithCode(code websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		c.hub.Unregister(c)
		c.sendMu.Lock()
		c.sendClosed = true
		close(c.send)
		c.sendMu.Unlock()
		_ = c.conn.Close(code, reason)
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
