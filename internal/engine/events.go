package engine

// Event types.
const (
	EventGameStarted       = "GAME_STARTED"
	EventPhaseChanged      = "PHASE_CHANGED"
	EventPreparationSubmit = "PREPARATION_SUBMITTED"
	EventRoundSubmitted    = "ROUND_SUBMITTED"
	EventVoteSubmitted     = "VOTE_SUBMITTED"
	EventRoundStarted      = "ROUND_STARTED"
	EventRoundFinished     = "ROUND_FINISHED"
	EventCycleFinished     = "CYCLE_FINISHED"
	EventCycleStarted      = "CYCLE_STARTED"
	EventGameFinished      = "GAME_FINISHED"
	EventPlayerLeft        = "PLAYER_LEFT"
	EventPlayerKicked      = "PLAYER_KICKED"
	EventHostTransferred   = "HOST_TRANSFERRED"
	EventForceResolved     = "FORCE_RESOLVED"
	EventTimeoutApplied    = "TIMEOUT_APPLIED"
	EventRoomReset         = "ROOM_RESET"
)

// Event describes a state change produced by applying a command.
type Event struct {
	Type     string
	Revision int
	Data     map[string]any
}
