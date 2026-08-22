package engine

import "time"

// Command types.
const (
	CommandStartGame         = "START_GAME"
	CommandSubmitPreparation = "SUBMIT_PREPARATION"
	CommandSubmitRoundMeme   = "SUBMIT_ROUND_MEME"
	CommandSubmitVote        = "SUBMIT_VOTE"
	CommandNextRound         = "NEXT_ROUND"
	CommandStartNextCycle    = "START_NEXT_CYCLE"
	CommandForceResolvePhase = "FORCE_RESOLVE_PHASE"
	CommandLeaveRoom         = "LEAVE_ROOM"
	CommandKickPlayer        = "KICK_PLAYER"
	CommandTimeoutPhase      = "TIMEOUT_PHASE"
)

// Command is a single action applied to the aggregate by the engine.
type Command struct {
	CommandID string
	Type      string
	// PlayerID is the acting player; empty for system/admin commands.
	PlayerID string
	// IsAdmin marks a Local Admin actor.
	IsAdmin bool
	Payload map[string]any
	Now     time.Time
}
