package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
)

// --- Rooms ---

func (q queries) CreateRoom(ctx context.Context, r room.Room) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO rooms (id, code, revision, state, created_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Code, r.Revision, string(r.State), r.CreatedAt.UTC().Format(timeFmt), nullableTime(r.ClosedAt))
	return err
}

func (q queries) GetRoom(ctx context.Context, id string) (room.Room, error) {
	return q.scanRoom(q.q.QueryRowContext(ctx, `
		SELECT id, code, revision, state, created_at, closed_at FROM rooms WHERE id = ?`, id))
}

func (q queries) GetRoomByCode(ctx context.Context, code string) (room.Room, error) {
	return q.scanRoom(q.q.QueryRowContext(ctx, `
		SELECT id, code, revision, state, created_at, closed_at FROM rooms WHERE code = ?`, code))
}

func (q queries) ListRoomsByState(ctx context.Context, state room.RoomState) ([]room.Room, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, code, revision, state, created_at, closed_at FROM rooms WHERE state = ?`, string(state))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []room.Room
	for rows.Next() {
		r, err := q.scanRoomRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q queries) ListRooms(ctx context.Context) ([]room.Room, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, code, revision, state, created_at, closed_at FROM rooms ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []room.Room
	for rows.Next() {
		r, err := q.scanRoomRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q queries) UpdateRoom(ctx context.Context, r room.Room) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE rooms SET code = ?, revision = ?, state = ?, created_at = ?, closed_at = ?
		WHERE id = ?`,
		r.Code, r.Revision, string(r.State), r.CreatedAt.UTC().Format(timeFmt), nullableTime(r.ClosedAt), r.ID)
	return err
}

// DeleteRoom removes a room and every row referencing it. Foreign keys are not
// declared ON DELETE CASCADE, so child rows must be deleted explicitly in
// FK-safe order.
func (q queries) DeleteRoom(ctx context.Context, roomID string) error {
	if err := q.DeleteGameByRoom(ctx, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM player_sessions WHERE player_id IN (SELECT id FROM players WHERE room_id = ?)`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `DELETE FROM players WHERE room_id = ?`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `DELETE FROM room_settings WHERE room_id = ?`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `DELETE FROM processed_commands WHERE room_id = ?`, roomID); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `DELETE FROM rooms WHERE id = ?`, roomID)
	return err
}

// DeleteGameByRoom removes all games of a room and their child rows. It is used
// both when deleting a room and when resetting a finished room to LOBBY.
func (q queries) DeleteGameByRoom(ctx context.Context, roomID string) error {
	// Dealt hands reference game_cycles of the room's games.
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM dealt_hands WHERE cycle_id IN (
			SELECT id FROM game_cycles WHERE game_id IN (SELECT id FROM games WHERE room_id = ?))`, roomID); err != nil {
		return err
	}
	// Round child rows reference the room's rounds.
	for _, child := range []string{"round_scores", "votes", "vote_options", "round_submissions"} {
		if _, err := q.q.ExecContext(ctx, `
			DELETE FROM `+child+` WHERE round_id IN (
				SELECT id FROM rounds WHERE game_id IN (SELECT id FROM games WHERE room_id = ?))`, roomID); err != nil {
			return err
		}
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM rounds WHERE game_id IN (SELECT id FROM games WHERE room_id = ?)`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM prepared_turns WHERE cycle_id IN (
			SELECT id FROM game_cycles WHERE game_id IN (SELECT id FROM games WHERE room_id = ?))`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM game_cycles WHERE game_id IN (SELECT id FROM games WHERE room_id = ?)`, roomID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM game_players WHERE game_id IN (SELECT id FROM games WHERE room_id = ?)`, roomID); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `DELETE FROM games WHERE room_id = ?`, roomID)
	return err
}

func (q queries) scanRoom(row *sql.Row) (room.Room, error) {
	return scanRoom(row)
}

func (q queries) scanRoomRows(row rowScanner) (room.Room, error) {
	return scanRoom(row)
}

func scanRoom(row rowScanner) (room.Room, error) {
	var r room.Room
	var state string
	var createdAt string
	var closedAt sql.NullString
	if err := row.Scan(&r.ID, &r.Code, &r.Revision, &state, &createdAt, &closedAt); err != nil {
		return room.Room{}, err
	}
	r.State = room.RoomState(state)
	r.CreatedAt = parseTime(createdAt)
	r.ClosedAt = parseNullableTime(closedAt)
	return r, nil
}

// --- Players ---

func (q queries) CreatePlayer(ctx context.Context, p player.Player) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO players (id, room_id, name, role, connected, joined_at, left_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.RoomID, p.Name, string(p.Role), boolInt(p.Connected), p.JoinedAt.UTC().Format(timeFmt), nullableTime(p.LeftAt))
	return err
}

func (q queries) GetPlayer(ctx context.Context, id string) (player.Player, error) {
	return q.scanPlayer(q.q.QueryRowContext(ctx, `
		SELECT id, room_id, name, role, connected, joined_at, left_at FROM players WHERE id = ?`, id))
}

func (q queries) ListPlayersByRoom(ctx context.Context, roomID string) ([]player.Player, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, room_id, name, role, connected, joined_at, left_at FROM players WHERE room_id = ?`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []player.Player
	for rows.Next() {
		p, err := q.scanPlayerRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q queries) UpdatePlayer(ctx context.Context, p player.Player) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE players SET room_id = ?, name = ?, role = ?, connected = ?, joined_at = ?, left_at = ?
		WHERE id = ?`,
		p.RoomID, p.Name, string(p.Role), boolInt(p.Connected), p.JoinedAt.UTC().Format(timeFmt), nullableTime(p.LeftAt), p.ID)
	return err
}

func (q queries) DeletePlayer(ctx context.Context, id string) error {
	_, err := q.q.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, id)
	return err
}

func (q queries) DeletePlayerSessionsByPlayer(ctx context.Context, playerID string) error {
	_, err := q.q.ExecContext(ctx, `DELETE FROM player_sessions WHERE player_id = ?`, playerID)
	return err
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func (q queries) scanPlayer(row *sql.Row) (player.Player, error) {
	return scanPlayer(row)
}

func (q queries) scanPlayerRows(row rowScanner) (player.Player, error) {
	return scanPlayer(row)
}

func scanPlayer(row rowScanner) (player.Player, error) {
	var p player.Player
	var role string
	var joinedAt string
	var leftAt sql.NullString
	if err := row.Scan(&p.ID, &p.RoomID, &p.Name, &role, &p.Connected, &joinedAt, &leftAt); err != nil {
		return player.Player{}, err
	}
	p.Role = player.Role(role)
	p.JoinedAt = parseTime(joinedAt)
	p.LeftAt = parseNullableTime(leftAt)
	return p, nil
}

// --- Sessions ---

func (q queries) CreateSession(ctx context.Context, s player.PlayerSession) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO player_sessions (id, player_id, token_hash, created_at, last_seen_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.PlayerID, s.TokenHash, s.CreatedAt.UTC().Format(timeFmt), nullableTime(s.LastSeenAt), nullableTime(s.RevokedAt))
	return err
}

func (q queries) GetSession(ctx context.Context, id string) (player.PlayerSession, error) {
	return q.scanSession(q.q.QueryRowContext(ctx, `
		SELECT id, player_id, token_hash, created_at, last_seen_at, revoked_at FROM player_sessions WHERE id = ?`, id))
}

func (q queries) GetSessionByTokenHash(ctx context.Context, tokenHash string) (player.PlayerSession, error) {
	return q.scanSession(q.q.QueryRowContext(ctx, `
		SELECT id, player_id, token_hash, created_at, last_seen_at, revoked_at FROM player_sessions WHERE token_hash = ?`, tokenHash))
}

func (q queries) ListSessionsByPlayer(ctx context.Context, playerID string) ([]player.PlayerSession, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, player_id, token_hash, created_at, last_seen_at, revoked_at FROM player_sessions WHERE player_id = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []player.PlayerSession
	for rows.Next() {
		s, err := q.scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q queries) UpdateSession(ctx context.Context, s player.PlayerSession) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE player_sessions SET player_id = ?, token_hash = ?, created_at = ?, last_seen_at = ?, revoked_at = ?
		WHERE id = ?`,
		s.PlayerID, s.TokenHash, s.CreatedAt.UTC().Format(timeFmt), nullableTime(s.LastSeenAt), nullableTime(s.RevokedAt), s.ID)
	return err
}

func (q queries) scanSession(row *sql.Row) (player.PlayerSession, error) {
	return scanSession(row)
}

func (q queries) scanSessionRows(row rowScanner) (player.PlayerSession, error) {
	return scanSession(row)
}

func scanSession(row rowScanner) (player.PlayerSession, error) {
	var s player.PlayerSession
	var createdAt string
	var lastSeen, revoked sql.NullString
	if err := row.Scan(&s.ID, &s.PlayerID, &s.TokenHash, &createdAt, &lastSeen, &revoked); err != nil {
		return player.PlayerSession{}, err
	}
	s.CreatedAt = parseTime(createdAt)
	s.LastSeenAt = parseNullableTime(lastSeen)
	s.RevokedAt = parseNullableTime(revoked)
	return s, nil
}

// --- Room settings ---

func (q queries) UpsertRoomSettings(ctx context.Context, roomID string, s room.RoomSettings) error {
	scoreJSON, err := json.Marshal(s.ScoreConfig)
	if err != nil {
		return fmt.Errorf("marshal score config: %w", err)
	}
	_, err = q.q.ExecContext(ctx, `
		INSERT INTO room_settings (
			room_id, min_players, max_players, hand_size,
			preparation_timeout_seconds, round_selection_timeout_seconds, voting_timeout_seconds,
			infinite_game, situation_separator, score_config)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(room_id) DO UPDATE SET
			min_players = excluded.min_players,
			max_players = excluded.max_players,
			hand_size = excluded.hand_size,
			preparation_timeout_seconds = excluded.preparation_timeout_seconds,
			round_selection_timeout_seconds = excluded.round_selection_timeout_seconds,
			voting_timeout_seconds = excluded.voting_timeout_seconds,
			infinite_game = excluded.infinite_game,
			situation_separator = excluded.situation_separator,
			score_config = excluded.score_config`,
		roomID, s.MinPlayers, s.MaxPlayers, s.HandSize,
		s.PreparationTimeoutSeconds, s.RoundSelectionTimeoutSeconds, s.VotingTimeoutSeconds,
		boolInt(s.InfiniteGame), s.SituationSeparator, string(scoreJSON))
	return err
}

func (q queries) GetRoomSettings(ctx context.Context, roomID string) (room.RoomSettings, error) {
	var s room.RoomSettings
	var infinite int
	var scoreJSON string
	err := q.q.QueryRowContext(ctx, `
		SELECT min_players, max_players, hand_size,
			preparation_timeout_seconds, round_selection_timeout_seconds, voting_timeout_seconds,
			infinite_game, situation_separator, score_config
		FROM room_settings WHERE room_id = ?`, roomID).
		Scan(&s.MinPlayers, &s.MaxPlayers, &s.HandSize,
			&s.PreparationTimeoutSeconds, &s.RoundSelectionTimeoutSeconds, &s.VotingTimeoutSeconds,
			&infinite, &s.SituationSeparator, &scoreJSON)
	if err != nil {
		return room.RoomSettings{}, err
	}
	s.InfiniteGame = infinite != 0
	if err := json.Unmarshal([]byte(scoreJSON), &s.ScoreConfig); err != nil {
		return room.RoomSettings{}, fmt.Errorf("unmarshal score config: %w", err)
	}
	return s, nil
}
