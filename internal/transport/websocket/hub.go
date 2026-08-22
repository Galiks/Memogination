// Package websocket provides the per-room WebSocket hub used to push state
// updates to connected clients.
package websocket

import (
	"encoding/json"
	"sync"
)

// Hub manages per-room client connections. It implements the coordinator's
// Broadcaster interface so room state changes are pushed to all clients.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*Client]struct{}
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{rooms: map[string]map[*Client]struct{}{}}
}

// Register adds a client to its room.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.roomID] == nil {
		h.rooms[c.roomID] = map[*Client]struct{}{}
	}
	h.rooms[c.roomID][c] = struct{}{}
}

// Unregister removes a client from its room, deleting the room entry when it
// becomes empty.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.rooms[c.roomID]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.rooms, c.roomID)
		}
	}
}

// Broadcast JSON-encodes msg and sends it to every client in the room. Slow
// clients that cannot accept the message are dropped (non-blocking send).
func (h *Hub) Broadcast(roomID string, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[roomID] {
		select {
		case c.send <- data:
		default:
		}
	}
}
