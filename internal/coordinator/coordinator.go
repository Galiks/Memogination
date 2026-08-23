// Package coordinator orchestrates rooms, sessions, the engine, and
// projections. It is the application service layer for the game.
package coordinator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/engine"
	"github.com/memomarium/memomarium/internal/projection"
	"github.com/memomarium/memomarium/internal/repository"
	"github.com/memomarium/memomarium/internal/session"
)

// Broadcaster is implemented by the transport layer to push WebSocket messages
// to a room.
type Broadcaster interface {
	Broadcast(roomID string, msg any)
}

// roomLock serializes commands for a single room.
type roomLock struct {
	mu sync.Mutex
}

// Coordinator is the application service for rooms and games.
type Coordinator struct {
	repo        repository.Repository
	engine      *engine.Engine
	sessions    *session.Service
	rooms       map[string]*roomLock
	mu          sync.Mutex
	broadcaster Broadcaster
}

// New returns a Coordinator wired to the given repository, engine, session
// service, and broadcaster. broadcaster may be nil.
func New(repo repository.Repository, eng *engine.Engine, sessions *session.Service, broadcaster Broadcaster) *Coordinator {
	return &Coordinator{
		repo:        repo,
		engine:      eng,
		sessions:    sessions,
		rooms:       map[string]*roomLock{},
		broadcaster: broadcaster,
	}
}

// lockFor returns the per-room lock, creating it on first use.
func (c *Coordinator) lockFor(roomID string) *roomLock {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.rooms[roomID]
	if !ok {
		l = &roomLock{}
		c.rooms[roomID] = l
	}
	return l
}

// CreateRoom creates a room with a unique join code and default settings.
func (c *Coordinator) CreateRoom(ctx context.Context, name string) (*room.Room, error) {
	if err := player.ValidateName(name); err != nil {
		return nil, engine.ErrInvalidName
	}
	r := &room.Room{
		ID:        uuid.NewString(),
		Revision:  0,
		State:     room.StateLobby,
		CreatedAt: time.Now().UTC(),
	}
	// Generate a unique code, retrying on collision.
	for i := 0; i < 10; i++ {
		code, err := room.NewCode()
		if err != nil {
			return nil, err
		}
		if _, err := c.repo.GetRoomByCode(ctx, code); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				r.Code = code
				break
			}
			return nil, err
		}
	}
	if r.Code == "" {
		return nil, engine.ErrInternal
	}
	err := c.repo.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.CreateRoom(ctx, *r); err != nil {
			return err
		}
		return tx.UpsertRoomSettings(ctx, r.ID, room.DefaultRoomSettings())
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// JoinRoom adds a player to a room, creating a session, and returns the raw
// session token plus the player's snapshot. The first player becomes HOST.
func (c *Coordinator) JoinRoom(ctx context.Context, code, name string) (string, *projection.GameSnapshot, error) {
	if err := player.ValidateName(name); err != nil {
		return "", nil, engine.ErrInvalidName
	}
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, engine.ErrRoomNotFound
		}
		return "", nil, err
	}
	if r.State == room.StateClosed {
		return "", nil, engine.ErrRoomFull
	}
	// Serialize joins with commands for this room so the player count and
	// room state cannot race (e.g. exceeding maxPlayers or joining mid-game).
	lock := c.lockFor(r.ID)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	var token string
	var playerID string
	err = c.repo.WithTx(ctx, func(tx repository.Tx) error {
		players, err := tx.ListPlayersByRoom(ctx, r.ID)
		if err != nil {
			return err
		}
		settings, err := tx.GetRoomSettings(ctx, r.ID)
		if err != nil {
			return err
		}
		// A player who left or was kicked (LeftAt set) no longer occupies a
		// seat: they must not count toward MaxPlayers or block their name.
		active := make([]player.Player, 0, len(players))
		for _, p := range players {
			if p.LeftAt == nil {
				active = append(active, p)
			}
		}
		if len(active) >= settings.MaxPlayers {
			return engine.ErrRoomFull
		}
		for _, p := range active {
			if strings.EqualFold(p.Name, name) {
				return engine.ErrInvalidName
			}
		}

		role := player.RolePlayer
		if len(players) == 0 {
			role = player.RoleHost
		}
		p := &player.Player{
			ID:        uuid.NewString(),
			RoomID:    r.ID,
			Name:      name,
			Role:      role,
			Connected: true,
			JoinedAt:  time.Now().UTC(),
		}
		if err := tx.CreatePlayer(ctx, *p); err != nil {
			return err
		}
		// Create the session within the same transaction so a player is never
		// persisted without a session (and vice versa).
		tok, err := session.NewToken()
		if err != nil {
			return err
		}
		sess := player.PlayerSession{
			ID:        uuid.NewString(),
			PlayerID:  p.ID,
			TokenHash: session.HashToken(tok),
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.CreateSession(ctx, sess); err != nil {
			return err
		}
		token = tok
		playerID = p.ID
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	agg, err := c.LoadAggregate(ctx, r.ID)
	if err != nil {
		return "", nil, err
	}
	if c.broadcaster != nil {
		c.broadcaster.Broadcast(r.ID, map[string]any{"type": "STATE_UPDATED"})
	}
	return token, projection.PlayerSnapshot(agg, playerID, false), nil
}

// Reconnect authenticates an existing session and marks the player connected.
// It never creates a new player.
func (c *Coordinator) Reconnect(ctx context.Context, code, token string) (*projection.GameSnapshot, error) {
	sess, err := c.sessions.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, engine.ErrRoomNotFound
		}
		return nil, err
	}
	// Serialize with commands for this room.
	lock := c.lockFor(r.ID)
	lock.mu.Lock()
	defer lock.mu.Unlock()
	p, err := c.repo.GetPlayer(ctx, sess.PlayerID)
	if err != nil {
		return nil, err
	}
	if p.RoomID != r.ID {
		return nil, engine.ErrPlayerNotFound
	}
	p.Connected = true
	if err := c.repo.UpdatePlayer(ctx, p); err != nil {
		return nil, err
	}
	agg, err := c.LoadAggregate(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	if c.broadcaster != nil {
		c.broadcaster.Broadcast(r.ID, map[string]any{"type": "STATE_UPDATED"})
	}
	return projection.PlayerSnapshot(agg, p.ID, false), nil
}

// MarkDisconnected marks a player as disconnected (connected=false) when their
// WebSocket connection fully closes. It preserves the player, score,
// submissions, votes, and queue: Disconnect != Leave.
func (c *Coordinator) MarkDisconnected(ctx context.Context, playerID string) error {
	p, err := c.repo.GetPlayer(ctx, playerID)
	if err != nil {
		return err
	}
	lock := c.lockFor(p.RoomID)
	lock.mu.Lock()
	defer lock.mu.Unlock()
	p.Connected = false
	if err := c.repo.UpdatePlayer(ctx, p); err != nil {
		return err
	}
	if c.broadcaster != nil {
		c.broadcaster.Broadcast(p.RoomID, map[string]any{"type": "STATE_UPDATED"})
	}
	return nil
}

// the result, records the processed command, and broadcasts the update.
func (c *Coordinator) HandleCommand(ctx context.Context, code string, cmd engine.Command, expectedRevision int, actorPlayerID string, isAdmin bool) ([]engine.Event, *projection.GameSnapshot, error) {
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, engine.ErrRoomNotFound
		}
		return nil, nil, err
	}
	lock := c.lockFor(r.ID)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	var events []engine.Event
	var snap *projection.GameSnapshot
	err = c.repo.WithTx(ctx, func(tx repository.Tx) error {
		if cmd.CommandID != "" {
			processed, err := tx.IsProcessedCommand(ctx, cmd.CommandID)
			if err != nil {
				return err
			}
			if processed {
				return engine.ErrCommandAlreadyProcessed
			}
		}
		agg, err := c.loadAggregate(ctx, tx, r.ID)
		if err != nil {
			return err
		}
		if expectedRevision >= 0 && agg.Room.Revision != expectedRevision {
			return stateChangedError(agg.Room.Revision)
		}
		cmd.PlayerID = actorPlayerID
		cmd.IsAdmin = isAdmin
		evs, err := c.engine.Handle(ctx, agg, cmd)
		if err != nil {
			return err
		}
		events = evs
		agg.Room.Revision++
		if err := c.SaveAggregate(ctx, tx, agg); err != nil {
			return err
		}
		if cmd.CommandID != "" {
			pc := repository.ProcessedCommand{
				CommandID:      cmd.CommandID,
				RoomID:         r.ID,
				PlayerID:       actorPlayerID,
				CommandType:    cmd.Type,
				ResultRevision: agg.Room.Revision,
				ProcessedAt:    time.Now().UTC(),
			}
			if err := tx.CreateProcessedCommand(ctx, pc); err != nil {
				return err
			}
		}
		snap = projection.PlayerSnapshot(agg, actorPlayerID, isAdmin)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if c.broadcaster != nil {
		c.broadcaster.Broadcast(r.ID, map[string]any{"type": "STATE_UPDATED", "revision": snap.Revision})
	}
	return events, snap, nil
}

// LoadAggregate loads all entities for a room into an Aggregate, reconstructing
// the game phase and deadline from the games table.
func (c *Coordinator) LoadAggregate(ctx context.Context, roomID string) (*engine.Aggregate, error) {
	return c.loadAggregate(ctx, c.repo, roomID)
}

func (c *Coordinator) loadAggregate(ctx context.Context, tx repository.Tx, roomID string) (*engine.Aggregate, error) {
	agg := &engine.Aggregate{}
	r, err := tx.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	agg.Room = &r
	settings, err := tx.GetRoomSettings(ctx, roomID)
	if err != nil {
		return nil, err
	}
	agg.Settings = settings

	players, err := tx.ListPlayersByRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	for i := range players {
		agg.Players = append(agg.Players, &players[i])
	}

	memes, err := tx.ListMemes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range memes {
		agg.Memes = append(agg.Memes, &memes[i])
	}
	situations, err := tx.ListSituations(ctx)
	if err != nil {
		return nil, err
	}
	for i := range situations {
		agg.Situations = append(agg.Situations, &situations[i])
	}

	g, err := tx.GetGameByRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No game yet: room is in LOBBY.
			return agg, nil
		}
		return nil, err
	}
	agg.Game = &g

	phase, deadline, err := tx.GetGamePhase(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	agg.Phase = phase
	agg.PhaseDeadlineAt = deadline

	gps, err := tx.ListGamePlayers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	for i := range gps {
		agg.GamePlayers = append(agg.GamePlayers, &gps[i])
	}

	cycles, err := tx.ListGameCycles(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	for i := range cycles {
		gc := cycles[i]
		if gc.ID == g.CurrentCycleID {
			agg.Cycle = &gc
		}
	}

	if agg.Cycle != nil {
		pts, err := tx.ListPreparedTurnsByCycle(ctx, agg.Cycle.ID)
		if err != nil {
			return nil, err
		}
		for i := range pts {
			agg.PreparedTurns = append(agg.PreparedTurns, &pts[i])
		}
		hands, err := tx.ListDealtHands(ctx, agg.Cycle.ID)
		if err != nil {
			return nil, err
		}
		agg.Hands = groupHands(hands)
	}

	rounds, err := tx.ListRoundsByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	for i := range rounds {
		rd := rounds[i]
		agg.Rounds = append(agg.Rounds, &rd)
		subs, err := tx.ListRoundSubmissions(ctx, rd.ID)
		if err != nil {
			return nil, err
		}
		for j := range subs {
			agg.Submissions = append(agg.Submissions, &subs[j])
		}
		opts, err := tx.ListVoteOptions(ctx, rd.ID)
		if err != nil {
			return nil, err
		}
		for j := range opts {
			agg.VoteOptions = append(agg.VoteOptions, &opts[j])
		}
		votes, err := tx.ListVotes(ctx, rd.ID)
		if err != nil {
			return nil, err
		}
		for j := range votes {
			agg.Votes = append(agg.Votes, &votes[j])
		}
		scores, err := tx.ListRoundScores(ctx, rd.ID)
		if err != nil {
			return nil, err
		}
		for j := range scores {
			agg.RoundScores = append(agg.RoundScores, &scores[j])
		}
	}
	return agg, nil
}

// SaveAggregate persists all aggregate entities (upsert) within the given
// transaction. It must be called inside a transaction.
func (c *Coordinator) SaveAggregate(ctx context.Context, tx repository.Tx, agg *engine.Aggregate) error {
	if err := tx.UpdateRoom(ctx, *agg.Room); err != nil {
		return err
	}
	if err := tx.UpsertRoomSettings(ctx, agg.Room.ID, agg.Settings); err != nil {
		return err
	}
	for _, p := range agg.Players {
		// A player who left or was kicked while there was no active game
		// (LOBBY) is removed entirely so their seat and name free up.
		if agg.Game == nil && p.LeftAt != nil {
			if err := tx.DeletePlayerSessionsByPlayer(ctx, p.ID); err != nil {
				return err
			}
			if err := tx.DeletePlayer(ctx, p.ID); err != nil {
				return err
			}
			continue
		}
		if err := tx.UpdatePlayer(ctx, *p); err != nil {
			return err
		}
	}
	if agg.Game == nil {
		// No game: this is either a brand-new room or a room that was reset
		// with START_NEW_GAME. In the latter case stale game rows would
		// resurrect the finished game on the next load, so remove them.
		return tx.DeleteGameByRoom(ctx, agg.Room.ID)
	}
	// The game may be new (START_GAME) or already persisted; upsert it.
	if _, err := tx.GetGame(ctx, agg.Game.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.CreateGame(ctx, *agg.Game); err != nil {
				return err
			}
		} else {
			return err
		}
	} else if err := tx.UpdateGame(ctx, *agg.Game); err != nil {
		return err
	}
	if err := tx.UpdateGamePhase(ctx, agg.Game.ID, agg.Phase, agg.PhaseDeadlineAt); err != nil {
		return err
	}

	// Replace per-game child rows so the DB matches the aggregate exactly.
	// Delete in FK-safe order: dealt_hands -> rounds+children -> cycles -> players.
	if err := tx.DeleteDealtHandsByGame(ctx, agg.Game.ID); err != nil {
		return fmt.Errorf("delete dealt hands: %w", err)
	}
	if err := tx.DeleteRoundsByGame(ctx, agg.Game.ID); err != nil {
		return fmt.Errorf("delete rounds: %w", err)
	}
	if err := tx.DeleteGameCyclesByGame(ctx, agg.Game.ID); err != nil {
		return fmt.Errorf("delete cycles: %w", err)
	}
	if err := tx.DeleteGamePlayersByGame(ctx, agg.Game.ID); err != nil {
		return fmt.Errorf("delete game players: %w", err)
	}

	for _, gp := range agg.GamePlayers {
		if err := tx.CreateGamePlayer(ctx, *gp); err != nil {
			return fmt.Errorf("create game player %s: %w", gp.ID, err)
		}
	}
	if agg.Cycle != nil {
		if err := tx.CreateGameCycle(ctx, *agg.Cycle); err != nil {
			return fmt.Errorf("create cycle %s: %w", agg.Cycle.ID, err)
		}
		for _, pt := range agg.PreparedTurns {
			if err := tx.CreatePreparedTurn(ctx, *pt); err != nil {
				return fmt.Errorf("create prepared turn %s: %w", pt.ID, err)
			}
		}
	}
	for _, r := range agg.Rounds {
		if err := tx.CreateRound(ctx, *r); err != nil {
			return fmt.Errorf("create round %s: %w", r.ID, err)
		}
	}
	// Hands reference rounds, so insert them after rounds.
	if agg.Cycle != nil {
		for _, h := range agg.Hands {
			if err := tx.AddDealtHands(ctx, agg.Cycle.ID, h.GamePlayerID, h.MemeIDs, h.Kind, h.RoundID); err != nil {
				return fmt.Errorf("add dealt hands %s/%s: %w", h.GamePlayerID, h.Kind, err)
			}
		}
	}
	for _, r := range agg.Rounds {
		for _, s := range agg.Submissions {
			if s.RoundID == r.ID {
				if err := tx.CreateRoundSubmission(ctx, *s); err != nil {
					return fmt.Errorf("create submission %s: %w", s.ID, err)
				}
			}
		}
		for _, o := range agg.VoteOptions {
			if o.RoundID == r.ID {
				if err := tx.CreateVoteOption(ctx, *o); err != nil {
					return err
				}
			}
		}
		for _, v := range agg.Votes {
			if v.RoundID == r.ID {
				if err := tx.CreateVote(ctx, *v); err != nil {
					return err
				}
			}
		}
		for _, rs := range agg.RoundScores {
			if rs.RoundID == r.ID {
				if err := tx.CreateRoundScore(ctx, *rs); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Recover applies deadline recovery on startup: for every IN_GAME room whose
// persisted phase deadline is in the past, it applies a TIMEOUT_PHASE command
// and persists the result.
func (c *Coordinator) Recover(ctx context.Context) error {
	rooms, err := c.repo.ListRoomsByState(ctx, room.StateInGame)
	if err != nil {
		return err
	}
	for _, r := range rooms {
		lock := c.lockFor(r.ID)
		lock.mu.Lock()
		_, err := c.applyTimeoutIfOverdue(ctx, r.ID)
		lock.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// StartTimeoutScheduler launches a goroutine that periodically applies
// TIMEOUT_PHASE to any IN_GAME room whose phase deadline has passed, so timers
// work during a live game (not just at startup). It stops when ctx is
// cancelled. Each room is serialized by its per-room lock and the check is
// idempotent (a fresh commandId per application), so it cannot double-fire.
func (c *Coordinator) StartTimeoutScheduler(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = c.CheckOverdueDeadlines(ctx)
			}
		}
	}()
}

// CheckOverdueDeadlines scans IN_GAME rooms and applies TIMEOUT_PHASE to any
// whose phase deadline is in the past. It is safe to call concurrently with
// commands because each room is serialized by its per-room lock.
func (c *Coordinator) CheckOverdueDeadlines(ctx context.Context) error {
	rooms, err := c.repo.ListRoomsByState(ctx, room.StateInGame)
	if err != nil {
		return err
	}
	for _, r := range rooms {
		lock := c.lockFor(r.ID)
		lock.mu.Lock()
		_, err := c.applyTimeoutIfOverdue(ctx, r.ID)
		lock.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// applyTimeoutIfOverdue applies a TIMEOUT_PHASE command to roomID if it has an
// active game whose phase deadline is in the past. It returns whether a
// timeout was applied. The caller must hold the room's lock.
func (c *Coordinator) applyTimeoutIfOverdue(ctx context.Context, roomID string) (bool, error) {
	cmd := engine.Command{
		CommandID: uuid.NewString(),
		Type:      engine.CommandTimeoutPhase,
		Now:       time.Now().UTC(),
	}
	var applied bool
	err := c.repo.WithTx(ctx, func(tx repository.Tx) error {
		agg, err := c.loadAggregate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if agg.Game == nil || agg.PhaseDeadlineAt == nil || !agg.PhaseDeadlineAt.Before(time.Now()) {
			return nil
		}
		switch agg.Phase {
		case game.PhasePreparation, game.PhaseRoundSelection, game.PhaseRoundVoting:
		default:
			return nil
		}
		if _, err := c.engine.Handle(ctx, agg, cmd); err != nil {
			return err
		}
		agg.Room.Revision++
		if err := c.SaveAggregate(ctx, tx, agg); err != nil {
			return err
		}
		pc := repository.ProcessedCommand{
			CommandID:      cmd.CommandID,
			RoomID:         roomID,
			CommandType:    cmd.Type,
			ResultRevision: agg.Room.Revision,
			ProcessedAt:    time.Now().UTC(),
		}
		if err := tx.CreateProcessedCommand(ctx, pc); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if applied && c.broadcaster != nil {
		c.broadcaster.Broadcast(roomID, map[string]any{"type": "STATE_UPDATED"})
	}
	return applied, nil
}

// GetRoomByCode returns the room with the given code.
func (c *Coordinator) GetRoomByCode(ctx context.Context, code string) (*room.Room, error) {
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, engine.ErrRoomNotFound
		}
		return nil, err
	}
	return &r, nil
}

// RoomSummary is the admin-facing representation of a room in the room
// management list.
type RoomSummary struct {
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	State       string     `json:"state"`
	Revision    int        `json:"revision"`
	CreatedAt   time.Time  `json:"createdAt"`
	ClosedAt    *time.Time `json:"closedAt"`
	PlayerCount int        `json:"playerCount"`
}

// ListRooms returns a summary of every room (oldest first). It is used by the
// admin room-management UI.
func (c *Coordinator) ListRooms(ctx context.Context) ([]RoomSummary, error) {
	rooms, err := c.repo.ListRooms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RoomSummary, 0, len(rooms))
	for _, r := range rooms {
		players, err := c.repo.ListPlayersByRoom(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		active := 0
		for _, p := range players {
			if p.LeftAt == nil {
				active++
			}
		}
		out = append(out, RoomSummary{
			ID:          r.ID,
			Code:        r.Code,
			State:       string(r.State),
			Revision:    r.Revision,
			CreatedAt:   r.CreatedAt,
			ClosedAt:    r.ClosedAt,
			PlayerCount: active,
		})
	}
	return out, nil
}

// DeleteRoom removes a room and all of its data. Admin only.
func (c *Coordinator) DeleteRoom(ctx context.Context, code string, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return engine.ErrRoomNotFound
		}
		return err
	}
	lock := c.lockFor(r.ID)
	lock.mu.Lock()
	defer lock.mu.Unlock()
	if err := c.repo.WithTx(ctx, func(tx repository.Tx) error {
		return tx.DeleteRoom(ctx, r.ID)
	}); err != nil {
		return err
	}
	delete(c.rooms, r.ID)
	if c.broadcaster != nil {
		c.broadcaster.Broadcast(r.ID, map[string]any{"type": "ROOM_DELETED"})
	}
	return nil
}

// GetSettings returns the room settings for the given code.
func (c *Coordinator) GetSettings(ctx context.Context, code string) (room.RoomSettings, error) {
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return room.RoomSettings{}, engine.ErrRoomNotFound
		}
		return room.RoomSettings{}, err
	}
	return c.repo.GetRoomSettings(ctx, r.ID)
}

// UpdateSettings updates room settings. Admin only, and only while the room is
// in LOBBY.
func (c *Coordinator) UpdateSettings(ctx context.Context, code string, settings room.RoomSettings, actorIsAdmin bool) error {
	if !actorIsAdmin {
		return engine.ErrNotAllowed
	}
	if err := settings.Validate(); err != nil {
		return engine.ErrInvalidSettings
	}
	r, err := c.repo.GetRoomByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return engine.ErrRoomNotFound
		}
		return err
	}
	if r.State != room.StateLobby {
		return engine.ErrInvalidPhase
	}
	return c.repo.UpsertRoomSettings(ctx, r.ID, settings)
}

// --- Content management ---

// ListMemes returns all memes.
func (c *Coordinator) ListMemes(ctx context.Context) ([]content.Meme, error) {
	return c.repo.ListMemes(ctx)
}

// GetMemeBySHA256 returns the meme with the given SHA-256, or sql.ErrNoRows if
// none exists. It is used to detect duplicate uploads.
func (c *Coordinator) GetMemeBySHA256(ctx context.Context, sha256 string) (content.Meme, error) {
	return c.repo.GetMemeBySHA256(ctx, sha256)
}

// GetMemeByOriginalFilename returns the meme with the given original filename,
// or sql.ErrNoRows if none exists. It is used to replace a meme on re-upload.
func (c *Coordinator) GetMemeByOriginalFilename(ctx context.Context, filename string) (content.Meme, error) {
	return c.repo.GetMemeByOriginalFilename(ctx, filename)
}

// AddMeme creates a meme. Admin only.
func (c *Coordinator) AddMeme(ctx context.Context, m content.Meme, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	return c.repo.CreateMeme(ctx, m)
}

// DeleteMeme removes a meme. Admin only.
func (c *Coordinator) DeleteMeme(ctx context.Context, id string, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	return c.repo.DeleteMeme(ctx, id)
}

// UpdateMeme updates a meme. Admin only.
func (c *Coordinator) UpdateMeme(ctx context.Context, m content.Meme, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	return c.repo.UpdateMeme(ctx, m)
}

// ListSituations returns all situations.
func (c *Coordinator) ListSituations(ctx context.Context) ([]content.Situation, error) {
	return c.repo.ListSituations(ctx)
}

// AddSituation creates a situation. Admin only.
func (c *Coordinator) AddSituation(ctx context.Context, s content.Situation, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	return c.repo.CreateSituation(ctx, s)
}

// DeleteSituation removes a situation. Admin only.
func (c *Coordinator) DeleteSituation(ctx context.Context, id string, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	return c.repo.DeleteSituation(ctx, id)
}

// UpdateSituation updates a situation. Admin only.
func (c *Coordinator) UpdateSituation(ctx context.Context, s content.Situation, isAdmin bool) error {
	if !isAdmin {
		return engine.ErrNotAllowed
	}
	return c.repo.UpdateSituation(ctx, s)
}

// BulkResult reports the outcome of a bulk situation import.
type BulkResult struct {
	Found      int `json:"found"`
	Duplicates int `json:"duplicates"`
	Added      int `json:"added"`
}

// BulkAddSituations parses a bulk situation text (situations separated by a
// delimiter line), deduplicates, and inserts them. Admin only.
func (c *Coordinator) BulkAddSituations(ctx context.Context, raw, delimiter string, isAdmin bool) (BulkResult, error) {
	if !isAdmin {
		return BulkResult{}, engine.ErrNotAllowed
	}
	items := parseBulk(raw, delimiter)
	seen := map[string]bool{}
	var unique []string
	for _, it := range items {
		if seen[it] {
			continue
		}
		seen[it] = true
		unique = append(unique, it)
	}
	result := BulkResult{Found: len(items), Duplicates: len(items) - len(unique)}
	for _, text := range unique {
		s := content.Situation{
			ID:        uuid.NewString(),
			Text:      text,
			Enabled:   true,
			Source:    "bulk",
			CreatedAt: time.Now().UTC(),
		}
		if err := c.repo.CreateSituation(ctx, s); err != nil {
			return result, err
		}
		result.Added++
	}
	return result, nil
}

// parseBulk normalizes line endings, splits the text into situations on the
// delimiter token anywhere in the text, trims, and removes empty entries.
func parseBulk(raw, delimiter string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if delimiter == "" {
		if text := strings.TrimSpace(normalized); text != "" {
			return []string{text}
		}
		return nil
	}
	var situations []string
	for _, part := range strings.Split(normalized, delimiter) {
		if text := strings.TrimSpace(part); text != "" {
			situations = append(situations, text)
		}
	}
	return situations
}

// groupHands groups dealt-hand rows into round.Hand values.
func groupHands(rows []repository.DealtHand) []*round.Hand {
	byKey := map[string]*round.Hand{}
	var order []string
	for _, h := range rows {
		key := h.GamePlayerID + "|" + h.Kind + "|" + h.RoundID
		hand, ok := byKey[key]
		if !ok {
			hand = &round.Hand{GamePlayerID: h.GamePlayerID, Kind: h.Kind, RoundID: h.RoundID}
			byKey[key] = hand
			order = append(order, key)
		}
		hand.MemeIDs = append(hand.MemeIDs, h.MemeID)
	}
	out := make([]*round.Hand, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// stateChangedError wraps ErrStateChanged with the current revision.
func stateChangedError(current int) error {
	return fmt.Errorf("%w: current revision %d", engine.ErrStateChanged, current)
}

// StateChangedRevision returns the current revision embedded in a
// STATE_CHANGED error, and whether err is a state-changed error. It is used by
// the transport layer to report the current revision on a 409 conflict.
func StateChangedRevision(err error) (int, bool) {
	if !errors.Is(err, engine.ErrStateChanged) {
		return 0, false
	}
	msg := err.Error()
	idx := strings.LastIndex(msg, "current revision ")
	if idx < 0 {
		return 0, true
	}
	n, perr := strconv.Atoi(strings.TrimSpace(msg[idx+len("current revision "):]))
	if perr != nil {
		return 0, true
	}
	return n, true
}
