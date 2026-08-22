package coordinator_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/memomarium/memomarium/internal/coordinator"
	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/engine"
	"github.com/memomarium/memomarium/internal/projection"
	"github.com/memomarium/memomarium/internal/repository/sqlite"
	"github.com/memomarium/memomarium/internal/session"
	storagesqlite "github.com/memomarium/memomarium/internal/storage/sqlite"
)

type broadcastMsg struct {
	roomID string
	msg    any
}

type fakeBroadcaster struct {
	mu   sync.Mutex
	msgs []broadcastMsg
}

func (f *fakeBroadcaster) Broadcast(roomID string, msg any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, broadcastMsg{roomID: roomID, msg: msg})
}

func (f *fakeBroadcaster) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.msgs)
}

// newCoordinator opens a fresh DB, seeds content, and returns a coordinator.
func newCoordinator(t *testing.T) (*coordinator.Coordinator, *sqlite.Repo) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storagesqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, storagesqlite.Migrate(db, dbPath, filepath.Join(dir, "backups")))

	repo := sqlite.New(db)
	eng := engine.New()
	sessions := session.NewService(repo)
	c := coordinator.New(repo, eng, sessions, &fakeBroadcaster{})

	ctx := context.Background()
	for i := 0; i < 30; i++ {
		m := content.Meme{
			ID:               uuid.NewString(),
			OriginalPath:     fmt.Sprintf("/o%d.png", i),
			ScreenPath:       fmt.Sprintf("/s%d.png", i),
			ThumbnailPath:    fmt.Sprintf("/t%d.png", i),
			OriginalFilename: fmt.Sprintf("o%d.png", i),
			MimeType:         "image/png",
			SHA256:           uuid.NewString(),
			Enabled:          true,
			Source:           "upload",
			CreatedAt:        time.Now().UTC(),
		}
		require.NoError(t, repo.CreateMeme(ctx, m))
	}
	for i := 0; i < 3; i++ {
		s := content.Situation{ID: uuid.NewString(), Text: fmt.Sprintf("Situation %d", i), Enabled: true, Source: "manual", CreatedAt: time.Now().UTC()}
		require.NoError(t, repo.CreateSituation(ctx, s))
	}
	return c, repo
}

type joinedPlayer struct {
	playerID string
	token    string
}

// joinPlayers joins the given names and returns their player IDs.
func joinPlayers(t *testing.T, c *coordinator.Coordinator, roomID, code string, names ...string) []joinedPlayer {
	t.Helper()
	ctx := context.Background()
	out := make([]joinedPlayer, 0, len(names))
	for _, name := range names {
		token, _, err := c.JoinRoom(ctx, code, name)
		require.NoError(t, err)
		out = append(out, joinedPlayer{playerID: playerIDForName(t, c, roomID, name), token: token})
	}
	return out
}

func playerIDForName(t *testing.T, c *coordinator.Coordinator, roomID, name string) string {
	t.Helper()
	ctx := context.Background()
	agg, err := c.LoadAggregate(ctx, roomID)
	require.NoError(t, err)
	for _, p := range agg.Players {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("player %q not found", name)
	return ""
}

func handFor(t *testing.T, c *coordinator.Coordinator, roomID, playerID, kind string) []string {
	t.Helper()
	ctx := context.Background()
	agg, err := c.LoadAggregate(ctx, roomID)
	require.NoError(t, err)
	gp := agg.GamePlayerByPlayerID(playerID)
	require.NotNil(t, gp)
	h := agg.HandFor(gp.ID, kind)
	require.NotNil(t, h)
	return h.MemeIDs
}

func activePlayerID(t *testing.T, c *coordinator.Coordinator, roomID string) string {
	t.Helper()
	ctx := context.Background()
	agg, err := c.LoadAggregate(ctx, roomID)
	require.NoError(t, err)
	r := agg.CurrentRound()
	require.NotNil(t, r)
	gp := agg.GamePlayerByID(r.ActiveGamePlayerID)
	require.NotNil(t, gp)
	return gp.PlayerID
}

func originalOption(t *testing.T, c *coordinator.Coordinator, roomID string) *round.VoteOption {
	t.Helper()
	ctx := context.Background()
	agg, err := c.LoadAggregate(ctx, roomID)
	require.NoError(t, err)
	r := agg.CurrentRound()
	require.NotNil(t, r)
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID && o.IsOriginal {
			return o
		}
	}
	t.Fatal("no original vote option")
	return nil
}

// flow tracks the room revision across commands.
type flow struct {
	t      *testing.T
	c      *coordinator.Coordinator
	roomID string
	code   string
	rev    int
}

func newFlow(t *testing.T, c *coordinator.Coordinator, roomID, code string) *flow {
	return &flow{t: t, c: c, roomID: roomID, code: code, rev: 0}
}

func (f *flow) cmd(commandID, cmdType string, actor string, isAdmin bool, payload map[string]any) *projection.GameSnapshot {
	f.t.Helper()
	_, snap, err := f.c.HandleCommand(context.Background(), f.code, engine.Command{
		Type:      cmdType,
		CommandID: commandID,
		Payload:   payload,
		Now:       time.Now().UTC(),
	}, f.rev, actor, isAdmin)
	require.NoError(f.t, err)
	f.rev = snap.Revision
	return snap
}

// phase returns the current game phase from the persisted aggregate.
func (f *flow) phase() string {
	f.t.Helper()
	agg, err := f.c.LoadAggregate(context.Background(), f.roomID)
	require.NoError(f.t, err)
	return string(agg.Phase)
}

func TestCreateRoomAndJoinFirstPlayerIsHost(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()

	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	require.NotEmpty(t, r.Code)
	require.Equal(t, room.StateLobby, r.State)

	token, snap, err := c.JoinRoom(ctx, r.Code, "Alice")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, "Alice", snap.Actor.Name)
	require.True(t, snap.Actor.IsHost)
	require.Equal(t, string(player.RoleHost), snap.Actor.Role)
}

func TestLobbySnapshotIncludesPlayersAndSettings(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()

	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)

	// No game has started yet: game must remain nil.
	_, snap, err := c.JoinRoom(ctx, r.Code, "Alice")
	require.NoError(t, err)
	require.Nil(t, snap.Game)
	require.Equal(t, string(room.StateLobby), snap.Room.State)

	// Settings are always present.
	require.Equal(t, room.DefaultRoomSettings().MinPlayers, snap.Settings.MinPlayers)

	// Players are always present, sorted by join time, with the host flagged.
	_, snap2, err := c.JoinRoom(ctx, r.Code, "Bob")
	require.NoError(t, err)
	require.Len(t, snap2.Players, 2)
	require.Equal(t, "Alice", snap2.Players[0].Name)
	require.True(t, snap2.Players[0].IsHost)
	require.Equal(t, string(player.RoleHost), snap2.Players[0].Role)
	require.Equal(t, "Bob", snap2.Players[1].Name)
	require.False(t, snap2.Players[1].IsHost)
	require.Equal(t, string(player.RolePlayer), snap2.Players[1].Role)
}

func TestJoinRoomDuplicateNameRejected(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)

	_, _, err = c.JoinRoom(ctx, r.Code, "Alice")
	require.NoError(t, err)
	// Case-insensitive duplicate.
	_, _, err = c.JoinRoom(ctx, r.Code, "alice")
	require.ErrorIs(t, err, engine.ErrInvalidName)
}

func TestJoinRoomFull(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)

	// Limit the room to 2 players.
	settings := room.DefaultRoomSettings()
	settings.MaxPlayers = 2
	require.NoError(t, c.UpdateSettings(ctx, r.Code, settings, true))

	_, _, err = c.JoinRoom(ctx, r.Code, "Alice")
	require.NoError(t, err)
	_, _, err = c.JoinRoom(ctx, r.Code, "Bob")
	require.NoError(t, err)
	_, _, err = c.JoinRoom(ctx, r.Code, "Carol")
	require.ErrorIs(t, err, engine.ErrRoomFull)
}

func TestStartGameHappyPathRevisionIncrements(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	joinPlayers(t, c, r.ID, r.Code, "Alice", "Bob")

	host := playerIDForName(t, c, r.ID, "Alice")
	f := newFlow(t, c, r.ID, r.Code)
	snap := f.cmd("cmd-start", engine.CommandStartGame, host, false, nil)
	require.Equal(t, string(game.PhasePreparation), snap.Phase)
	require.Equal(t, string(room.StateInGame), snap.Room.State)
	require.NotNil(t, snap.Game)
	require.Greater(t, snap.Revision, 0)
	require.Equal(t, snap.Revision, f.rev)
}

func TestStaleExpectedRevision(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	joinPlayers(t, c, r.ID, r.Code, "Alice", "Bob")
	host := playerIDForName(t, c, r.ID, "Alice")

	// Start game with expectedRevision 0 (correct).
	_, snap, err := c.HandleCommand(ctx, r.Code, engine.Command{Type: engine.CommandStartGame, CommandID: "cmd-1", Now: time.Now().UTC()}, 0, host, false)
	require.NoError(t, err)

	// Now issue a command with a stale expectedRevision.
	_, _, err = c.HandleCommand(ctx, r.Code, engine.Command{Type: engine.CommandSubmitPreparation, CommandID: "cmd-2", Payload: map[string]any{"situationText": "Situation 0", "memeId": "x"}, Now: time.Now().UTC()}, 0, host, false)
	require.ErrorIs(t, err, engine.ErrStateChanged)
	require.Equal(t, "STATE_CHANGED", engine.Code(err))
	_ = snap
}

func TestDuplicateCommandIDRejected(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	joinPlayers(t, c, r.ID, r.Code, "Alice", "Bob")
	host := playerIDForName(t, c, r.ID, "Alice")

	_, _, err = c.HandleCommand(ctx, r.Code, engine.Command{Type: engine.CommandStartGame, CommandID: "dup", Now: time.Now().UTC()}, 0, host, false)
	require.NoError(t, err)

	// Replaying the same command ID must be rejected.
	_, _, err = c.HandleCommand(ctx, r.Code, engine.Command{Type: engine.CommandStartGame, CommandID: "dup", Now: time.Now().UTC()}, 0, host, false)
	require.ErrorIs(t, err, engine.ErrCommandAlreadyProcessed)
}

func TestFullGameFlow(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	players := joinPlayers(t, c, r.ID, r.Code, "Alice", "Bob", "Carol", "Dave")
	host := players[0].playerID

	f := newFlow(t, c, r.ID, r.Code)

	// Start.
	f.cmd("s", engine.CommandStartGame, host, false, nil)

	// Prepare all 4 players.
	for _, p := range players {
		hand := handFor(t, c, r.ID, p.playerID, engine.HandKindPreparation)
		f.cmd("prep-"+p.playerID, engine.CommandSubmitPreparation, p.playerID, false, map[string]any{
			"situationText": "Situation 0",
			"memeId":        hand[0],
		})
	}
	require.Equal(t, string(game.PhaseRoundSelection), f.phase())

	// Play 4 rounds (one per active player).
	for roundIdx := 0; roundIdx < 4; roundIdx++ {
		active := activePlayerID(t, c, r.ID)
		// Non-active players submit a round meme.
		for _, p := range players {
			if p.playerID == active {
				continue
			}
			hand := handFor(t, c, r.ID, p.playerID, engine.HandKindRound)
			f.cmd("sub-"+p.playerID+"-"+fmt.Sprint(roundIdx), engine.CommandSubmitRoundMeme, p.playerID, false, map[string]any{"memeId": hand[0]})
		}
		require.Equal(t, string(game.PhaseRoundVoting), f.phase())

		// Non-active players vote for the original.
		orig := originalOption(t, c, r.ID)
		for _, p := range players {
			if p.playerID == active {
				continue
			}
			f.cmd("vote-"+p.playerID+"-"+fmt.Sprint(roundIdx), engine.CommandSubmitVote, p.playerID, false, map[string]any{"voteOptionId": orig.ID})
		}
		require.Equal(t, string(game.PhaseRoundResults), f.phase())

		// Advance to the next round (or finish the game on the last round).
		if roundIdx < 3 {
			f.cmd("next-"+fmt.Sprint(roundIdx), engine.CommandNextRound, host, false, nil)
		}
	}

	// After the 4th round, NEXT_ROUND finishes the game.
	snap := f.cmd("next-final", engine.CommandNextRound, host, false, nil)
	require.Equal(t, string(game.PhaseGameResults), snap.Phase)
	require.Equal(t, string(game.StateFinished), snap.Game.State)
	require.Equal(t, string(room.StateClosed), snap.Room.State)
	require.NotEmpty(t, snap.Game.Leaderboard)
}

func TestRestartRecovery(t *testing.T) {
	c1, repo := newCoordinator(t)
	ctx := context.Background()
	r, err := c1.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	players := joinPlayers(t, c1, r.ID, r.Code, "Alice", "Bob", "Carol", "Dave")
	host := players[0].playerID

	f := newFlow(t, c1, r.ID, r.Code)
	f.cmd("s", engine.CommandStartGame, host, false, nil)
	for _, p := range players {
		hand := handFor(t, c1, r.ID, p.playerID, engine.HandKindPreparation)
		f.cmd("prep-"+p.playerID, engine.CommandSubmitPreparation, p.playerID, false, map[string]any{"situationText": "Situation 0", "memeId": hand[0]})
	}
	active := activePlayerID(t, c1, r.ID)
	for _, p := range players {
		if p.playerID == active {
			continue
		}
		hand := handFor(t, c1, r.ID, p.playerID, engine.HandKindRound)
		f.cmd("sub-"+p.playerID, engine.CommandSubmitRoundMeme, p.playerID, false, map[string]any{"memeId": hand[0]})
	}
	require.Equal(t, string(game.PhaseRoundVoting), f.phase())

	// Build a NEW coordinator over the same DB.
	c2 := coordinator.New(repo, engine.New(), session.NewService(repo), &fakeBroadcaster{})
	require.NoError(t, c2.Recover(ctx))

	// Phase must be reconstructed as ROUND_VOTING.
	agg, err := c2.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)

	// The game continues: vote and reach ROUND_RESULTS.
	f2 := newFlow(t, c2, r.ID, r.Code)
	f2.rev = agg.Room.Revision
	orig := originalOption(t, c2, r.ID)
	for _, p := range players {
		if p.playerID == active {
			continue
		}
		f2.cmd("vote-"+p.playerID, engine.CommandSubmitVote, p.playerID, false, map[string]any{"voteOptionId": orig.ID})
	}
	require.Equal(t, string(game.PhaseRoundResults), f2.phase())
}

func TestDeadlineRecovery(t *testing.T) {
	c1, repo := newCoordinator(t)
	ctx := context.Background()
	r, err := c1.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	players := joinPlayers(t, c1, r.ID, r.Code, "Alice", "Bob")
	host := players[0].playerID

	f := newFlow(t, c1, r.ID, r.Code)
	f.cmd("s", engine.CommandStartGame, host, false, nil)
	require.Equal(t, string(game.PhasePreparation), f.phase())

	// Set a past phase deadline.
	agg, err := c1.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	past := time.Now().Add(-time.Hour)
	require.NoError(t, repo.UpdateGamePhase(ctx, agg.Game.ID, agg.Phase, &past))

	// A new coordinator recovers and applies TIMEOUT_PHASE.
	c2 := coordinator.New(repo, engine.New(), session.NewService(repo), &fakeBroadcaster{})
	require.NoError(t, c2.Recover(ctx))

	agg2, err := c2.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	// TIMEOUT_PHASE on PREPARATION auto-prepares and advances to ROUND_SELECTION.
	require.Equal(t, game.PhaseRoundSelection, agg2.Phase)
	require.Len(t, agg2.PreparedTurns, 2)
}

func TestBulkAddSituations(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()

	raw := "First situation\r\nsecond line\r\n*\r\nSecond situation\r\n*\r\nFirst situation\r\n\r\n"
	res, err := c.BulkAddSituations(ctx, raw, "*", true)
	require.NoError(t, err)
	require.Equal(t, 3, res.Found)
	// Dedupe is on exact normalized text; all three differ, so none are dropped.
	require.Equal(t, 0, res.Duplicates)
	require.Equal(t, 3, res.Added)

	// Non-admin rejected.
	_, err = c.BulkAddSituations(ctx, raw, "*", false)
	require.ErrorIs(t, err, engine.ErrNotAllowed)
}

func TestProjectionSecurityVoting(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	players := joinPlayers(t, c, r.ID, r.Code, "Alice", "Bob", "Carol")
	host := players[0].playerID

	f := newFlow(t, c, r.ID, r.Code)
	f.cmd("s", engine.CommandStartGame, host, false, nil)
	for _, p := range players {
		hand := handFor(t, c, r.ID, p.playerID, engine.HandKindPreparation)
		f.cmd("prep-"+p.playerID, engine.CommandSubmitPreparation, p.playerID, false, map[string]any{"situationText": "Situation 0", "memeId": hand[0]})
	}
	active := activePlayerID(t, c, r.ID)
	for _, p := range players {
		if p.playerID == active {
			continue
		}
		hand := handFor(t, c, r.ID, p.playerID, engine.HandKindRound)
		f.cmd("sub-"+p.playerID, engine.CommandSubmitRoundMeme, p.playerID, false, map[string]any{"memeId": hand[0]})
	}
	require.Equal(t, string(game.PhaseRoundVoting), f.phase())

	// A non-active player's snapshot must not reveal owners or the original.
	voter := players[1]
	agg, err := c.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	snap := projection.PlayerSnapshot(agg, voter.playerID, false)
	opts, ok := snap.PhaseData["voteOptions"].([]projection.VoteOptionDTO)
	require.True(t, ok)
	require.NotEmpty(t, opts)
	// VoteOptionDTO is anonymous by construction: it only carries id/number/memeId.
	for _, o := range opts {
		require.NotEmpty(t, o.ID)
		require.NotEmpty(t, o.MemeID)
		require.Greater(t, o.Number, 0)
	}
	// The forbidden option (the voter's own) must be present.
	require.NotEmpty(t, snap.PhaseData["forbiddenOptionId"])
	// No private hands leaked.
	_, hasHand := snap.PhaseData["hand"]
	require.False(t, hasHand)
}

func TestConcurrentJoinsRespectMaxPlayers(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	r, err := c.CreateRoom(ctx, "Host")
	require.NoError(t, err)

	settings := room.DefaultRoomSettings()
	settings.MaxPlayers = 5
	require.NoError(t, c.UpdateSettings(ctx, r.Code, settings, true))

	const attempts = 20
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := c.JoinRoom(ctx, r.Code, fmt.Sprintf("P%d", i))
			errs[i] = err
		}(i)
	}
	wg.Wait()

	agg, err := c.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	// The per-room lock serializes joins, so the player count never exceeds
	// maxPlayers even under concurrent load.
	require.LessOrEqual(t, len(agg.Players), 5)
}

func TestCheckOverdueDeadlinesAdvancesPhase(t *testing.T) {
	c1, repo := newCoordinator(t)
	ctx := context.Background()
	r, err := c1.CreateRoom(ctx, "Host")
	require.NoError(t, err)
	players := joinPlayers(t, c1, r.ID, r.Code, "Alice", "Bob")
	host := players[0].playerID

	f := newFlow(t, c1, r.ID, r.Code)
	f.cmd("s", engine.CommandStartGame, host, false, nil)
	require.Equal(t, string(game.PhasePreparation), f.phase())

	// Set a past phase deadline.
	agg, err := c1.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	past := time.Now().Add(-time.Hour)
	require.NoError(t, repo.UpdateGamePhase(ctx, agg.Game.ID, agg.Phase, &past))

	// The scheduler's check applies TIMEOUT_PHASE and advances the phase.
	require.NoError(t, c1.CheckOverdueDeadlines(ctx))
	agg2, err := c1.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundSelection, agg2.Phase)
	require.Len(t, agg2.PreparedTurns, 2)

	// Running again must not double-fire (deadline is now in the future/nil).
	require.NoError(t, c1.CheckOverdueDeadlines(ctx))
	agg3, err := c1.LoadAggregate(ctx, r.ID)
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundSelection, agg3.Phase)
}
