// Package player defines the player and session domain models.
package player

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Role is the role of a player within a room.
type Role string

const (
	RoleHost   Role = "HOST"
	RolePlayer Role = "PLAYER"
)

// Player is a participant in a room.
type Player struct {
	ID        string     `json:"id"`
	RoomID    string     `json:"roomId"`
	Name      string     `json:"name"`
	Role      Role       `json:"role"`
	Connected bool       `json:"connected"`
	JoinedAt  time.Time  `json:"joinedAt"`
	LeftAt    *time.Time `json:"leftAt"`
}

// ValidateName validates a player display name: trimmed, non-empty, at most 32
// characters.
func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("name must not be empty")
	}
	if len([]rune(trimmed)) > 32 {
		return fmt.Errorf("name must be at most 32 characters, got %d", len([]rune(trimmed)))
	}
	return nil
}
