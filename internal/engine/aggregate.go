package engine

import (
	"time"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
)

// Hand kinds used to distinguish preparation hands from per-round hands.
const (
	HandKindPreparation = "PREPARATION"
	HandKindRound       = "ROUND"
)

// Aggregate is the in-memory aggregate the engine operates on. The engine
// mutates it in place and returns events describing the resulting changes.
type Aggregate struct {
	Room          *room.Room
	Settings      room.RoomSettings
	Players       []*player.Player
	Game          *game.Game
	GamePlayers   []*game.GamePlayer
	Cycle         *game.GameCycle
	PreparedTurns []*round.PreparedTurn
	Rounds        []*round.Round
	Hands         []*round.Hand
	Submissions   []*round.RoundSubmission
	VoteOptions   []*round.VoteOption
	Votes         []*round.Vote
	RoundScores   []*round.RoundScore
	Memes         []*content.Meme
	Situations    []*content.Situation

	// Phase is the current game phase. It is an in-memory-only concept not
	// persisted to the database.
	Phase game.GamePhase
	// PhaseDeadlineAt is the deadline for the current phase, if any.
	PhaseDeadlineAt *time.Time
}

// ActiveGamePlayers returns game players whose participation status is ACTIVE.
func (a *Aggregate) ActiveGamePlayers() []*game.GamePlayer {
	var out []*game.GamePlayer
	for _, gp := range a.GamePlayers {
		if gp.ParticipationStatus == game.ParticipationActive {
			out = append(out, gp)
		}
	}
	return out
}

// ActivePlayers returns room players who have not left.
func (a *Aggregate) ActivePlayers() []*player.Player {
	var out []*player.Player
	for _, p := range a.Players {
		if p.LeftAt == nil {
			out = append(out, p)
		}
	}
	return out
}

// PlayerByID returns the room player with the given ID, or nil.
func (a *Aggregate) PlayerByID(id string) *player.Player {
	for _, p := range a.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// GamePlayerByPlayerID returns the game player for the given room player ID.
func (a *Aggregate) GamePlayerByPlayerID(playerID string) *game.GamePlayer {
	for _, gp := range a.GamePlayers {
		if gp.PlayerID == playerID {
			return gp
		}
	}
	return nil
}

// GamePlayerByID returns the game player with the given game player ID.
func (a *Aggregate) GamePlayerByID(id string) *game.GamePlayer {
	for _, gp := range a.GamePlayers {
		if gp.ID == id {
			return gp
		}
	}
	return nil
}

// CurrentRound returns the round referenced by Game.CurrentRoundID, or nil.
func (a *Aggregate) CurrentRound() *round.Round {
	if a.Game == nil || a.Game.CurrentRoundID == "" {
		return nil
	}
	for _, r := range a.Rounds {
		if r.ID == a.Game.CurrentRoundID {
			return r
		}
	}
	return nil
}

// PreparedTurnFor returns the prepared turn for the given game player, or nil.
func (a *Aggregate) PreparedTurnFor(gamePlayerID string) *round.PreparedTurn {
	for _, pt := range a.PreparedTurns {
		if pt.GamePlayerID == gamePlayerID {
			return pt
		}
	}
	return nil
}

// HandFor returns the hand of the given kind for the given game player. For
// round hands it returns the hand belonging to the current round.
func (a *Aggregate) HandFor(gamePlayerID, kind string) *round.Hand {
	for _, h := range a.Hands {
		if h.GamePlayerID != gamePlayerID || h.Kind != kind {
			continue
		}
		if kind == HandKindRound {
			if a.Game != nil && h.RoundID == a.Game.CurrentRoundID {
				return h
			}
			continue
		}
		return h
	}
	return nil
}

// EnabledMemes returns the IDs of enabled memes.
func (a *Aggregate) EnabledMemes() []string {
	var out []string
	for _, m := range a.Memes {
		if m.Enabled {
			out = append(out, m.ID)
		}
	}
	return out
}

// EnabledSituations returns the enabled situations.
func (a *Aggregate) EnabledSituations() []*content.Situation {
	var out []*content.Situation
	for _, s := range a.Situations {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}
