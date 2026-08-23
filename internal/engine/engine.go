package engine

import "context"

// Engine implements the game state machine. It is pure-ish: it operates on an
// in-memory aggregate, mutating it in place and returning events. It does not
// touch HTTP/WS/DB directly.
type Engine struct {
	dealer *MemeDealer
}

// New returns an Engine with a deterministically seeded MemeDealer.
func New() *Engine {
	return &Engine{dealer: NewMemeDealer(1)}
}

// NewWithDealer returns an Engine using the given MemeDealer.
func NewWithDealer(dealer *MemeDealer) *Engine {
	return &Engine{dealer: dealer}
}

// Handle dispatches a command to the appropriate transition, mutating the
// aggregate in place and returning the resulting events.
func (e *Engine) Handle(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	switch cmd.Type {
	case CommandStartGame:
		return e.startGame(ctx, agg, cmd)
	case CommandSubmitPreparation:
		return e.submitPreparation(ctx, agg, cmd)
	case CommandSubmitRoundMeme:
		return e.submitRoundMeme(ctx, agg, cmd)
	case CommandSubmitVote:
		return e.submitVote(ctx, agg, cmd)
	case CommandNextRound:
		return e.nextRound(ctx, agg, cmd)
	case CommandStartNextCycle:
		return e.startNextCycle(ctx, agg, cmd)
	case CommandForceResolvePhase:
		return e.forceResolvePhase(ctx, agg, cmd)
	case CommandLeaveRoom:
		return e.leaveRoom(ctx, agg, cmd)
	case CommandKickPlayer:
		return e.kickPlayer(ctx, agg, cmd)
	case CommandTimeoutPhase:
		return e.timeoutPhase(ctx, agg, cmd)
	case CommandStartNewGame:
		return e.startNewGame(ctx, agg, cmd)
	default:
		return nil, ErrInvalidCommand
	}
}
