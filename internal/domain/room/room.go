// Package room defines the room domain model and room settings.
package room

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/memomarium/memomarium/internal/domain/scoring"
)

// RoomState is the lifecycle state of a room.
type RoomState string

const (
	StateLobby  RoomState = "LOBBY"
	StateInGame RoomState = "IN_GAME"
	StateClosed RoomState = "CLOSED"
)

// Room is a joinable game room.
type Room struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Revision  int        `json:"revision"`
	State     RoomState  `json:"state"`
	CreatedAt time.Time  `json:"createdAt"`
	ClosedAt  *time.Time `json:"closedAt"`
}

// RoomSettings holds the configurable rules for a room.
type RoomSettings struct {
	MinPlayers                   int                 `json:"minPlayers"`
	MaxPlayers                   int                 `json:"maxPlayers"`
	HandSize                     int                 `json:"handSize"`
	PreparationTimeoutSeconds    int                 `json:"preparationTimeoutSeconds"`
	RoundSelectionTimeoutSeconds int                 `json:"roundSelectionTimeoutSeconds"`
	VotingTimeoutSeconds         int                 `json:"votingTimeoutSeconds"`
	InfiniteGame                 bool                `json:"infiniteGame"`
	SituationSeparator           string              `json:"situationSeparator"`
	ScoreConfig                  scoring.ScoreConfig `json:"scoreConfig"`
}

// DefaultRoomSettings returns the standard room settings.
func DefaultRoomSettings() RoomSettings {
	return RoomSettings{
		MinPlayers:         2,
		MaxPlayers:         10,
		HandSize:           5,
		InfiniteGame:       false,
		SituationSeparator: "*",
		ScoreConfig:        scoring.DefaultScoreConfig(),
	}
}

// Validate checks the settings against the allowed ranges.
func (s RoomSettings) Validate() error {
	if s.MinPlayers < 2 || s.MinPlayers > 20 {
		return fmt.Errorf("min_players must be between 2 and 20, got %d", s.MinPlayers)
	}
	if s.MaxPlayers < 2 || s.MaxPlayers > 20 {
		return fmt.Errorf("max_players must be between 2 and 20, got %d", s.MaxPlayers)
	}
	if s.MinPlayers > s.MaxPlayers {
		return fmt.Errorf("min_players (%d) must not exceed max_players (%d)", s.MinPlayers, s.MaxPlayers)
	}
	if s.HandSize < 3 || s.HandSize > 10 {
		return fmt.Errorf("hand_size must be between 3 and 10, got %d", s.HandSize)
	}
	for name, v := range map[string]int{
		"preparation_timeout_seconds":     s.PreparationTimeoutSeconds,
		"round_selection_timeout_seconds": s.RoundSelectionTimeoutSeconds,
		"voting_timeout_seconds":          s.VotingTimeoutSeconds,
	} {
		if v < 0 || v > 3600 {
			return fmt.Errorf("%s must be between 0 and 3600, got %d", name, v)
		}
	}
	return nil
}

// codeAlphabet excludes ambiguous characters (0, O, 1, I, L).
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const codeLength = 6

// NewCode generates a random 6-character room join code using crypto/rand.
func NewCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate room code: %w", err)
	}
	for i := range buf {
		buf[i] = codeAlphabet[int(buf[i])%len(codeAlphabet)]
	}
	return string(buf), nil
}

// ErrEmptyName is returned when a name is empty after trimming.
var ErrEmptyName = errors.New("name must not be empty")
