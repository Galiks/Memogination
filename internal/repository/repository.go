// Package repository defines the persistence interface for the game domain.
package repository

import (
	"context"
	"time"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
)

// ProcessedCommand records a command that has already been applied, enabling
// idempotent command processing.
type ProcessedCommand struct {
	CommandID      string
	RoomID         string
	PlayerID       string
	CommandType    string
	ResultRevision int
	ProcessedAt    time.Time
}

// Tx is a transaction-scoped repository. All methods operate within the
// enclosing transaction.
type Tx interface {
	// Rooms.
	CreateRoom(ctx context.Context, r room.Room) error
	GetRoom(ctx context.Context, id string) (room.Room, error)
	GetRoomByCode(ctx context.Context, code string) (room.Room, error)
	ListRoomsByState(ctx context.Context, state room.RoomState) ([]room.Room, error)
	UpdateRoom(ctx context.Context, r room.Room) error

	// Players.
	CreatePlayer(ctx context.Context, p player.Player) error
	GetPlayer(ctx context.Context, id string) (player.Player, error)
	ListPlayersByRoom(ctx context.Context, roomID string) ([]player.Player, error)
	UpdatePlayer(ctx context.Context, p player.Player) error

	// Sessions.
	CreateSession(ctx context.Context, s player.PlayerSession) error
	GetSession(ctx context.Context, id string) (player.PlayerSession, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (player.PlayerSession, error)
	ListSessionsByPlayer(ctx context.Context, playerID string) ([]player.PlayerSession, error)
	UpdateSession(ctx context.Context, s player.PlayerSession) error

	// Room settings.
	UpsertRoomSettings(ctx context.Context, roomID string, s room.RoomSettings) error
	GetRoomSettings(ctx context.Context, roomID string) (room.RoomSettings, error)

	// Games.
	CreateGame(ctx context.Context, g game.Game) error
	GetGame(ctx context.Context, id string) (game.Game, error)
	GetGameByRoom(ctx context.Context, roomID string) (game.Game, error)
	UpdateGame(ctx context.Context, g game.Game) error
	// GetGamePhase returns the persisted game phase and its deadline.
	GetGamePhase(ctx context.Context, gameID string) (game.GamePhase, *time.Time, error)
	// UpdateGamePhase persists the game phase and its deadline.
	UpdateGamePhase(ctx context.Context, gameID string, phase game.GamePhase, deadlineAt *time.Time) error

	// Game players.
	CreateGamePlayer(ctx context.Context, gp game.GamePlayer) error
	GetGamePlayer(ctx context.Context, id string) (game.GamePlayer, error)
	ListGamePlayers(ctx context.Context, gameID string) ([]game.GamePlayer, error)
	UpdateGamePlayer(ctx context.Context, gp game.GamePlayer) error

	// Game cycles.
	CreateGameCycle(ctx context.Context, gc game.GameCycle) error
	GetGameCycle(ctx context.Context, id string) (game.GameCycle, error)
	ListGameCycles(ctx context.Context, gameID string) ([]game.GameCycle, error)
	UpdateGameCycle(ctx context.Context, gc game.GameCycle) error

	// Prepared turns.
	CreatePreparedTurn(ctx context.Context, pt round.PreparedTurn) error
	GetPreparedTurn(ctx context.Context, id string) (round.PreparedTurn, error)
	ListPreparedTurnsByCycle(ctx context.Context, cycleID string) ([]round.PreparedTurn, error)
	UpdatePreparedTurn(ctx context.Context, pt round.PreparedTurn) error

	// Rounds.
	CreateRound(ctx context.Context, r round.Round) error
	GetRound(ctx context.Context, id string) (round.Round, error)
	ListRoundsByGame(ctx context.Context, gameID string) ([]round.Round, error)
	UpdateRound(ctx context.Context, r round.Round) error

	// Dealt hands.
	AddDealtHands(ctx context.Context, cycleID, gamePlayerID string, memeIDs []string, kind, roundID string) error
	ListDealtHands(ctx context.Context, cycleID string) ([]DealtHand, error)

	// Round submissions.
	CreateRoundSubmission(ctx context.Context, s round.RoundSubmission) error
	GetRoundSubmission(ctx context.Context, id string) (round.RoundSubmission, error)
	ListRoundSubmissions(ctx context.Context, roundID string) ([]round.RoundSubmission, error)

	// Vote options.
	CreateVoteOption(ctx context.Context, vo round.VoteOption) error
	ListVoteOptions(ctx context.Context, roundID string) ([]round.VoteOption, error)

	// Votes.
	CreateVote(ctx context.Context, v round.Vote) error
	ListVotes(ctx context.Context, roundID string) ([]round.Vote, error)

	// Round scores.
	CreateRoundScore(ctx context.Context, rs round.RoundScore) error
	ListRoundScores(ctx context.Context, roundID string) ([]round.RoundScore, error)

	// Game child-row deletion (used by SaveAggregate to replace game state).
	DeleteDealtHandsByGame(ctx context.Context, gameID string) error
	DeleteRoundsByGame(ctx context.Context, gameID string) error
	DeleteGameCyclesByGame(ctx context.Context, gameID string) error
	DeleteGamePlayersByGame(ctx context.Context, gameID string) error

	// Processed commands.
	CreateProcessedCommand(ctx context.Context, pc ProcessedCommand) error
	IsProcessedCommand(ctx context.Context, commandID string) (bool, error)

	// Memes.
	CreateMeme(ctx context.Context, m content.Meme) error
	GetMeme(ctx context.Context, id string) (content.Meme, error)
	GetMemeBySHA256(ctx context.Context, sha256 string) (content.Meme, error)
	GetMemeByOriginalFilename(ctx context.Context, filename string) (content.Meme, error)
	ListMemes(ctx context.Context) ([]content.Meme, error)
	UpdateMeme(ctx context.Context, m content.Meme) error
	DeleteMeme(ctx context.Context, id string) error

	// Situations.
	CreateSituation(ctx context.Context, s content.Situation) error
	GetSituation(ctx context.Context, id string) (content.Situation, error)
	ListSituations(ctx context.Context) ([]content.Situation, error)
	UpdateSituation(ctx context.Context, s content.Situation) error
	DeleteSituation(ctx context.Context, id string) error
}

// DealtHand is a row from the dealt_hands table.
type DealtHand struct {
	ID           string
	CycleID      string
	GamePlayerID string
	MemeID       string
	Kind         string
	RoundID      string
}

// Repository is the persistence interface for the whole game.
type Repository interface {
	Tx
	// WithTx runs fn within a transaction. If fn returns an error the
	// transaction is rolled back; otherwise it is committed.
	WithTx(ctx context.Context, fn func(tx Tx) error) error
}
