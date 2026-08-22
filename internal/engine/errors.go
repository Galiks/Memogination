// Package engine implements the pure-ish game state machine for Memomarium.
package engine

import (
	"errors"
	"fmt"
)

// EngineError is an engine error carrying a stable API error code.
type EngineError struct {
	code string
	msg  string
}

func (e *EngineError) Error() string { return e.msg }

// Code returns the stable API error code.
func (e *EngineError) Code() string { return e.code }

func newError(code, msg string) *EngineError {
	return &EngineError{code: code, msg: msg}
}

// Sentinel engine errors matching the API error codes.
var (
	ErrRoomNotFound            = newError("ROOM_NOT_FOUND", "room not found")
	ErrRoomFull                = newError("ROOM_FULL", "room is full")
	ErrGameAlreadyStarted      = newError("GAME_ALREADY_STARTED", "game already started")
	ErrGameNotStarted          = newError("GAME_NOT_STARTED", "game not started")
	ErrPlayerNotFound          = newError("PLAYER_NOT_FOUND", "player not found")
	ErrPlayerAlreadyLeft       = newError("PLAYER_ALREADY_LEFT", "player already left")
	ErrInvalidSession          = newError("INVALID_SESSION", "invalid session")
	ErrNotAllowed              = newError("NOT_ALLOWED", "not allowed")
	ErrInvalidCommand          = newError("INVALID_COMMAND", "invalid command")
	ErrInvalidPhase            = newError("INVALID_PHASE", "invalid phase")
	ErrStateChanged            = newError("STATE_CHANGED", "state changed")
	ErrCommandAlreadyProcessed = newError("COMMAND_ALREADY_PROCESSED", "command already processed")
	ErrInvalidName             = newError("INVALID_NAME", "invalid name")
	ErrInvalidSettings         = newError("INVALID_SETTINGS", "invalid settings")
	ErrNotEnoughMemes          = newError("NOT_ENOUGH_MEMES", "not enough memes")
	ErrNotEnoughSituations     = newError("NOT_ENOUGH_SITUATIONS", "not enough situations")
	ErrInvalidMeme             = newError("INVALID_MEME", "invalid meme")
	ErrInvalidVote             = newError("INVALID_VOTE", "invalid vote")
	ErrOwnMemeVoteForbidden    = newError("OWN_MEME_VOTE_FORBIDDEN", "voting for own meme is forbidden")
	ErrFileTooLarge            = newError("FILE_TOO_LARGE", "file too large")
	ErrDuplicateMeme           = newError("DUPLICATE_MEME", "meme already exists")
	ErrInternal                = newError("INTERNAL", "internal error")
)

// Code returns the stable API error code for err, or "" if err is not an
// engine error.
func Code(err error) string {
	var ee *EngineError
	if errors.As(err, &ee) {
		return ee.code
	}
	return ""
}

// wrapf wraps a sentinel engine error with a formatted message while
// preserving its API error code and enabling errors.Is matching.
func wrapf(err *EngineError, format string, args ...any) error {
	return fmt.Errorf("%w: %s", err, fmt.Sprintf(format, args...))
}
