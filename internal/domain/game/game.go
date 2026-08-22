// Package game defines the game, game player, and game cycle domain models.
package game

import (
	"time"

	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/scoring"
)

// GamePhase is the phase of a round or cycle.
type GamePhase string

const (
	PhasePreparation    GamePhase = "PREPARATION"
	PhaseRoundSelection GamePhase = "ROUND_SELECTION"
	PhaseRoundVoting    GamePhase = "ROUND_VOTING"
	PhaseRoundResults   GamePhase = "ROUND_RESULTS"
	PhaseCycleResults   GamePhase = "CYCLE_RESULTS"
	PhaseGameResults    GamePhase = "GAME_RESULTS"
)

// GameState is the lifecycle state of a game.
type GameState string

const (
	StateActive   GameState = "ACTIVE"
	StateFinished GameState = "FINISHED"
)

// Game is a single play session within a room.
type Game struct {
	ID               string               `json:"id"`
	RoomID           string               `json:"roomId"`
	State            GameState            `json:"state"`
	Revision         int                  `json:"revision"`
	SettingsSnapshot GameSettingsSnapshot `json:"settingsSnapshot"`
	CurrentCycleID   string               `json:"currentCycleId"`
	CurrentRoundID   string               `json:"currentRoundId"`
	StartedAt        time.Time            `json:"startedAt"`
	FinishedAt       *time.Time           `json:"finishedAt"`
}

// GameSettingsSnapshot is an immutable copy of the room settings captured when
// the game started.
type GameSettingsSnapshot struct {
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

// NewGameSettingsSnapshot builds an immutable snapshot from room settings.
func NewGameSettingsSnapshot(s room.RoomSettings) GameSettingsSnapshot {
	return GameSettingsSnapshot{
		MinPlayers:                   s.MinPlayers,
		MaxPlayers:                   s.MaxPlayers,
		HandSize:                     s.HandSize,
		PreparationTimeoutSeconds:    s.PreparationTimeoutSeconds,
		RoundSelectionTimeoutSeconds: s.RoundSelectionTimeoutSeconds,
		VotingTimeoutSeconds:         s.VotingTimeoutSeconds,
		InfiniteGame:                 s.InfiniteGame,
		SituationSeparator:           s.SituationSeparator,
		ScoreConfig:                  s.ScoreConfig,
	}
}

// ParticipationStatus is the participation state of a game player.
type ParticipationStatus string

const (
	ParticipationActive  ParticipationStatus = "ACTIVE"
	ParticipationLeft    ParticipationStatus = "LEFT"
	ParticipationKicked  ParticipationStatus = "KICKED"
	ParticipationSkipped ParticipationStatus = "SKIPPED"
)

// GamePlayer is a player's participation in a specific game.
type GamePlayer struct {
	ID                  string              `json:"id"`
	GameID              string              `json:"gameId"`
	PlayerID            string              `json:"playerId"`
	DisplayName         string              `json:"displayName"`
	TurnOrder           int                 `json:"turnOrder"`
	Score               int                 `json:"score"`
	ParticipationStatus ParticipationStatus `json:"participationStatus"`
}

// GameCycle is one cycle (round of turns) within a game.
type GameCycle struct {
	ID         string     `json:"id"`
	GameID     string     `json:"gameId"`
	Number     int        `json:"number"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}
