package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/repository"
	"github.com/memomarium/memomarium/internal/storage/sqlite"
)

// newTestDB opens a fresh SQLite database in a temp dir and applies migrations.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Migrate directly (no backup logic needed for tests).
	require.NoError(t, sqlite.Migrate(db, dbPath, filepath.Join(dir, "backups")))
	return db
}

func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

func newRoom() room.Room {
	return room.Room{
		ID:        uuid.NewString(),
		Code:      "ABC234",
		Revision:  0,
		State:     room.StateLobby,
		CreatedAt: now(),
	}
}

func newPlayer(roomID string) player.Player {
	return player.Player{
		ID:        uuid.NewString(),
		RoomID:    roomID,
		Name:      "Alice",
		Role:      player.RolePlayer,
		Connected: true,
		JoinedAt:  now(),
	}
}

func newMeme() content.Meme {
	return content.Meme{
		ID:               uuid.NewString(),
		OriginalPath:     "/orig.png",
		ScreenPath:       "/screen.png",
		ThumbnailPath:    "/thumb.png",
		OriginalFilename: "orig.png",
		MimeType:         "image/png",
		SHA256:           uuid.NewString(),
		Enabled:          true,
		Source:           "upload",
		CreatedAt:        now(),
	}
}

func TestMigrationsRun(t *testing.T) {
	db := newTestDB(t)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='rooms'`).Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='memes'`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestForeignKeyEnforced(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	// Inserting a player referencing a non-existent room must fail.
	err := repo.CreatePlayer(ctx, newPlayer("no-such-room"))
	require.Error(t, err)
}

func TestDuplicatePreparedTurnRejected(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))
	p := newPlayer(r.ID)
	require.NoError(t, repo.CreatePlayer(ctx, p))

	g := game.Game{
		ID:        uuid.NewString(),
		RoomID:    r.ID,
		State:     game.StateActive,
		Revision:  0,
		StartedAt: now(),
	}
	require.NoError(t, repo.CreateGame(ctx, g))

	gp := game.GamePlayer{
		ID:                  uuid.NewString(),
		GameID:              g.ID,
		PlayerID:            p.ID,
		DisplayName:         p.Name,
		TurnOrder:           0,
		Score:               0,
		ParticipationStatus: game.ParticipationActive,
	}
	require.NoError(t, repo.CreateGamePlayer(ctx, gp))

	cycle := game.GameCycle{ID: uuid.NewString(), GameID: g.ID, Number: 1, StartedAt: now()}
	require.NoError(t, repo.CreateGameCycle(ctx, cycle))

	pt := round.PreparedTurn{
		ID:           uuid.NewString(),
		CycleID:      cycle.ID,
		GamePlayerID: gp.ID,
		Status:       "PENDING",
		CreatedAt:    now(),
	}
	require.NoError(t, repo.CreatePreparedTurn(ctx, pt))

	dup := pt
	dup.ID = uuid.NewString()
	err := repo.CreatePreparedTurn(ctx, dup)
	require.Error(t, err)
}

func TestDuplicateVoteRejected(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))
	p1 := newPlayer(r.ID)
	p1.Name = "P1"
	require.NoError(t, repo.CreatePlayer(ctx, p1))
	p2 := newPlayer(r.ID)
	p2.Name = "P2"
	require.NoError(t, repo.CreatePlayer(ctx, p2))

	g := game.Game{ID: uuid.NewString(), RoomID: r.ID, State: game.StateActive, StartedAt: now()}
	require.NoError(t, repo.CreateGame(ctx, g))
	gp1 := game.GamePlayer{ID: uuid.NewString(), GameID: g.ID, PlayerID: p1.ID, DisplayName: p1.Name, TurnOrder: 0, ParticipationStatus: game.ParticipationActive}
	gp2 := game.GamePlayer{ID: uuid.NewString(), GameID: g.ID, PlayerID: p2.ID, DisplayName: p2.Name, TurnOrder: 1, ParticipationStatus: game.ParticipationActive}
	require.NoError(t, repo.CreateGamePlayer(ctx, gp1))
	require.NoError(t, repo.CreateGamePlayer(ctx, gp2))

	cycle := game.GameCycle{ID: uuid.NewString(), GameID: g.ID, Number: 1, StartedAt: now()}
	require.NoError(t, repo.CreateGameCycle(ctx, cycle))

	meme := newMeme()
	require.NoError(t, repo.CreateMeme(ctx, meme))

	rd := round.Round{
		ID:                 uuid.NewString(),
		GameID:             g.ID,
		CycleID:            cycle.ID,
		ActiveGamePlayerID: gp1.ID,
		Phase:              "ROUND_VOTING",
		SituationText:      "situation",
		OriginalMemeID:     meme.ID,
		Status:             "ACTIVE",
		StartedAt:          now(),
	}
	require.NoError(t, repo.CreateRound(ctx, rd))

	vo := round.VoteOption{ID: uuid.NewString(), RoundID: rd.ID, Number: 1, MemeID: meme.ID, IsOriginal: true}
	require.NoError(t, repo.CreateVoteOption(ctx, vo))

	v := round.Vote{ID: uuid.NewString(), RoundID: rd.ID, GamePlayerID: gp2.ID, VoteOptionID: vo.ID, CreatedAt: now()}
	require.NoError(t, repo.CreateVote(ctx, v))

	dup := v
	dup.ID = uuid.NewString()
	err := repo.CreateVote(ctx, dup)
	require.Error(t, err)
}

func TestDuplicateProcessedCommandRejected(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))

	pc := repository.ProcessedCommand{
		CommandID:      "cmd-1",
		RoomID:         r.ID,
		CommandType:    "CREATE_ROOM",
		ResultRevision: 1,
		ProcessedAt:    now(),
	}
	require.NoError(t, repo.CreateProcessedCommand(ctx, pc))

	dup := pc
	dup.ProcessedAt = now()
	err := repo.CreateProcessedCommand(ctx, dup)
	require.Error(t, err)

	ok, err := repo.IsProcessedCommand(ctx, "cmd-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestTransactionRollback(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))

	// Inside a transaction, create a player then return an error to roll back.
	err := repo.WithTx(ctx, func(tx repository.Tx) error {
		require.NoError(t, tx.CreatePlayer(ctx, newPlayer(r.ID)))
		return errors.New("boom")
	})
	require.Error(t, err)

	players, err := repo.ListPlayersByRoom(ctx, r.ID)
	require.NoError(t, err)
	assert.Empty(t, players)
}

func TestRoomPersistenceRoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))

	got, err := repo.GetRoom(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, r.Code, got.Code)
	assert.Equal(t, r.Revision, got.Revision)
	assert.Equal(t, r.State, got.State)
	assert.WithinDuration(t, r.CreatedAt, got.CreatedAt, time.Second)

	byCode, err := repo.GetRoomByCode(ctx, r.Code)
	require.NoError(t, err)
	assert.Equal(t, r.ID, byCode.ID)

	// Update and reload.
	r.Revision = 1
	r.State = room.StateInGame
	require.NoError(t, repo.UpdateRoom(ctx, r))
	got, err = repo.GetRoom(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Revision)
	assert.Equal(t, room.StateInGame, got.State)
}

func TestGamePersistenceRoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))

	settings := room.DefaultRoomSettings()
	settings.HandSize = 7
	require.NoError(t, repo.UpsertRoomSettings(ctx, r.ID, settings))

	g := game.Game{
		ID:               uuid.NewString(),
		RoomID:           r.ID,
		State:            game.StateActive,
		Revision:         0,
		SettingsSnapshot: game.NewGameSettingsSnapshot(settings),
		StartedAt:        now(),
	}
	require.NoError(t, repo.CreateGame(ctx, g))

	got, err := repo.GetGame(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
	assert.Equal(t, g.RoomID, got.RoomID)
	assert.Equal(t, g.State, got.State)
	assert.Equal(t, 7, got.SettingsSnapshot.HandSize)
	assert.Equal(t, settings.ScoreConfig, got.SettingsSnapshot.ScoreConfig)
	assert.WithinDuration(t, g.StartedAt, got.StartedAt, time.Second)

	// Reload settings too.
	gotSettings, err := repo.GetRoomSettings(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, gotSettings.HandSize)
	assert.Equal(t, settings.ScoreConfig, gotSettings.ScoreConfig)
}

func TestRoomSettingsUpsert(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	r := newRoom()
	require.NoError(t, repo.CreateRoom(ctx, r))

	s1 := room.DefaultRoomSettings()
	s1.MaxPlayers = 8
	require.NoError(t, repo.UpsertRoomSettings(ctx, r.ID, s1))

	s2 := room.DefaultRoomSettings()
	s2.MaxPlayers = 6
	require.NoError(t, repo.UpsertRoomSettings(ctx, r.ID, s2))

	got, err := repo.GetRoomSettings(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, 6, got.MaxPlayers)
}

func TestGetMemeByOriginalFilename(t *testing.T) {
	db := newTestDB(t)
	repo := New(db)
	ctx := context.Background()

	m := newMeme()
	require.NoError(t, repo.CreateMeme(ctx, m))

	got, err := repo.GetMemeByOriginalFilename(ctx, m.OriginalFilename)
	require.NoError(t, err)
	assert.Equal(t, m.ID, got.ID)

	_, err = repo.GetMemeByOriginalFilename(ctx, "missing.png")
	require.ErrorIs(t, err, sql.ErrNoRows)
}
