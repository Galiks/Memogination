package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/repository"
)

// --- Games ---

func (q queries) CreateGame(ctx context.Context, g game.Game) error {
	snapJSON, err := json.Marshal(g.SettingsSnapshot)
	if err != nil {
		return fmt.Errorf("marshal settings snapshot: %w", err)
	}
	_, err = q.q.ExecContext(ctx, `
		INSERT INTO games (id, room_id, state, revision, settings_snapshot, current_cycle_id, current_round_id, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.RoomID, string(g.State), g.Revision, string(snapJSON),
		nullableString(g.CurrentCycleID), nullableString(g.CurrentRoundID),
		g.StartedAt.UTC().Format(timeFmt), nullableTime(g.FinishedAt))
	return err
}

func (q queries) GetGame(ctx context.Context, id string) (game.Game, error) {
	var g game.Game
	var state string
	var snapJSON string
	var startedAt string
	var cycleID, roundID, finishedAt sql.NullString
	err := q.q.QueryRowContext(ctx, `
		SELECT id, room_id, state, revision, settings_snapshot, current_cycle_id, current_round_id, started_at, finished_at
		FROM games WHERE id = ?`, id).
		Scan(&g.ID, &g.RoomID, &state, &g.Revision, &snapJSON, &cycleID, &roundID, &startedAt, &finishedAt)
	if err != nil {
		return game.Game{}, err
	}
	g.State = game.GameState(state)
	g.StartedAt = parseTime(startedAt)
	g.CurrentCycleID = cycleID.String
	g.CurrentRoundID = roundID.String
	g.FinishedAt = parseNullableTime(finishedAt)
	if err := json.Unmarshal([]byte(snapJSON), &g.SettingsSnapshot); err != nil {
		return game.Game{}, fmt.Errorf("unmarshal settings snapshot: %w", err)
	}
	return g, nil
}

func (q queries) GetGameByRoom(ctx context.Context, roomID string) (game.Game, error) {
	var g game.Game
	var state string
	var snapJSON string
	var startedAt string
	var cycleID, roundID, finishedAt sql.NullString
	err := q.q.QueryRowContext(ctx, `
		SELECT id, room_id, state, revision, settings_snapshot, current_cycle_id, current_round_id, started_at, finished_at
		FROM games WHERE room_id = ? ORDER BY started_at DESC LIMIT 1`, roomID).
		Scan(&g.ID, &g.RoomID, &state, &g.Revision, &snapJSON, &cycleID, &roundID, &startedAt, &finishedAt)
	if err != nil {
		return game.Game{}, err
	}
	g.State = game.GameState(state)
	g.StartedAt = parseTime(startedAt)
	g.CurrentCycleID = cycleID.String
	g.CurrentRoundID = roundID.String
	g.FinishedAt = parseNullableTime(finishedAt)
	if err := json.Unmarshal([]byte(snapJSON), &g.SettingsSnapshot); err != nil {
		return game.Game{}, fmt.Errorf("unmarshal settings snapshot: %w", err)
	}
	return g, nil
}

func (q queries) UpdateGame(ctx context.Context, g game.Game) error {
	snapJSON, err := json.Marshal(g.SettingsSnapshot)
	if err != nil {
		return fmt.Errorf("marshal settings snapshot: %w", err)
	}
	_, err = q.q.ExecContext(ctx, `
		UPDATE games SET room_id = ?, state = ?, revision = ?, settings_snapshot = ?,
			current_cycle_id = ?, current_round_id = ?, started_at = ?, finished_at = ?
		WHERE id = ?`,
		g.RoomID, string(g.State), g.Revision, string(snapJSON),
		nullableString(g.CurrentCycleID), nullableString(g.CurrentRoundID),
		g.StartedAt.UTC().Format(timeFmt), nullableTime(g.FinishedAt), g.ID)
	return err
}

func (q queries) GetGamePhase(ctx context.Context, gameID string) (game.GamePhase, *time.Time, error) {
	var phase sql.NullString
	var deadline sql.NullString
	err := q.q.QueryRowContext(ctx, `
		SELECT phase, phase_deadline_at FROM games WHERE id = ?`, gameID).
		Scan(&phase, &deadline)
	if err != nil {
		return "", nil, err
	}
	return game.GamePhase(phase.String), parseNullableTime(deadline), nil
}

func (q queries) UpdateGamePhase(ctx context.Context, gameID string, phase game.GamePhase, deadlineAt *time.Time) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE games SET phase = ?, phase_deadline_at = ? WHERE id = ?`,
		nullableString(string(phase)), nullableTime(deadlineAt), gameID)
	return err
}

// --- Game players ---

func (q queries) CreateGamePlayer(ctx context.Context, gp game.GamePlayer) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO game_players (id, game_id, player_id, display_name, turn_order, score, participation_status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		gp.ID, gp.GameID, gp.PlayerID, gp.DisplayName, gp.TurnOrder, gp.Score, string(gp.ParticipationStatus))
	return err
}

func (q queries) GetGamePlayer(ctx context.Context, id string) (game.GamePlayer, error) {
	return q.scanGamePlayer(q.q.QueryRowContext(ctx, `
		SELECT id, game_id, player_id, display_name, turn_order, score, participation_status FROM game_players WHERE id = ?`, id))
}

func (q queries) ListGamePlayers(ctx context.Context, gameID string) ([]game.GamePlayer, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, game_id, player_id, display_name, turn_order, score, participation_status FROM game_players WHERE game_id = ?`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.GamePlayer
	for rows.Next() {
		gp, err := q.scanGamePlayerRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, gp)
	}
	return out, rows.Err()
}

func (q queries) UpdateGamePlayer(ctx context.Context, gp game.GamePlayer) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE game_players SET game_id = ?, player_id = ?, display_name = ?, turn_order = ?, score = ?, participation_status = ?
		WHERE id = ?`,
		gp.GameID, gp.PlayerID, gp.DisplayName, gp.TurnOrder, gp.Score, string(gp.ParticipationStatus), gp.ID)
	return err
}

func (q queries) scanGamePlayer(row *sql.Row) (game.GamePlayer, error) {
	return scanGamePlayer(row)
}

func (q queries) scanGamePlayerRows(row rowScanner) (game.GamePlayer, error) {
	return scanGamePlayer(row)
}

func scanGamePlayer(row rowScanner) (game.GamePlayer, error) {
	var gp game.GamePlayer
	var status string
	if err := row.Scan(&gp.ID, &gp.GameID, &gp.PlayerID, &gp.DisplayName, &gp.TurnOrder, &gp.Score, &status); err != nil {
		return game.GamePlayer{}, err
	}
	gp.ParticipationStatus = game.ParticipationStatus(status)
	return gp, nil
}

// --- Game cycles ---

func (q queries) CreateGameCycle(ctx context.Context, gc game.GameCycle) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO game_cycles (id, game_id, number, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)`,
		gc.ID, gc.GameID, gc.Number, gc.StartedAt.UTC().Format(timeFmt), nullableTime(gc.FinishedAt))
	return err
}

func (q queries) GetGameCycle(ctx context.Context, id string) (game.GameCycle, error) {
	var gc game.GameCycle
	var startedAt string
	var finishedAt sql.NullString
	err := q.q.QueryRowContext(ctx, `
		SELECT id, game_id, number, started_at, finished_at FROM game_cycles WHERE id = ?`, id).
		Scan(&gc.ID, &gc.GameID, &gc.Number, &startedAt, &finishedAt)
	if err != nil {
		return game.GameCycle{}, err
	}
	gc.StartedAt = parseTime(startedAt)
	gc.FinishedAt = parseNullableTime(finishedAt)
	return gc, nil
}

func (q queries) ListGameCycles(ctx context.Context, gameID string) ([]game.GameCycle, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, game_id, number, started_at, finished_at FROM game_cycles WHERE game_id = ?`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.GameCycle
	for rows.Next() {
		var gc game.GameCycle
		var startedAt string
		var finishedAt sql.NullString
		if err := rows.Scan(&gc.ID, &gc.GameID, &gc.Number, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		gc.StartedAt = parseTime(startedAt)
		gc.FinishedAt = parseNullableTime(finishedAt)
		out = append(out, gc)
	}
	return out, rows.Err()
}

func (q queries) UpdateGameCycle(ctx context.Context, gc game.GameCycle) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE game_cycles SET game_id = ?, number = ?, started_at = ?, finished_at = ? WHERE id = ?`,
		gc.GameID, gc.Number, gc.StartedAt.UTC().Format(timeFmt), nullableTime(gc.FinishedAt), gc.ID)
	return err
}

// --- Prepared turns ---

func (q queries) CreatePreparedTurn(ctx context.Context, pt round.PreparedTurn) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO prepared_turns (id, cycle_id, game_player_id, situation_text, original_meme_id, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		pt.ID, pt.CycleID, pt.GamePlayerID, nullableString(pt.SituationText), nullableString(pt.OriginalMemeID),
		pt.Status, pt.CreatedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) GetPreparedTurn(ctx context.Context, id string) (round.PreparedTurn, error) {
	return q.scanPreparedTurn(q.q.QueryRowContext(ctx, `
		SELECT id, cycle_id, game_player_id, situation_text, original_meme_id, status, created_at FROM prepared_turns WHERE id = ?`, id))
}

func (q queries) ListPreparedTurnsByCycle(ctx context.Context, cycleID string) ([]round.PreparedTurn, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, cycle_id, game_player_id, situation_text, original_meme_id, status, created_at FROM prepared_turns WHERE cycle_id = ?`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.PreparedTurn
	for rows.Next() {
		pt, err := q.scanPreparedTurnRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

func (q queries) UpdatePreparedTurn(ctx context.Context, pt round.PreparedTurn) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE prepared_turns SET cycle_id = ?, game_player_id = ?, situation_text = ?, original_meme_id = ?, status = ?, created_at = ?
		WHERE id = ?`,
		pt.CycleID, pt.GamePlayerID, nullableString(pt.SituationText), nullableString(pt.OriginalMemeID),
		pt.Status, pt.CreatedAt.UTC().Format(timeFmt), pt.ID)
	return err
}

func (q queries) scanPreparedTurn(row *sql.Row) (round.PreparedTurn, error) {
	return scanPreparedTurn(row)
}

func (q queries) scanPreparedTurnRows(row rowScanner) (round.PreparedTurn, error) {
	return scanPreparedTurn(row)
}

func scanPreparedTurn(row rowScanner) (round.PreparedTurn, error) {
	var pt round.PreparedTurn
	var situation, meme sql.NullString
	var createdAt string
	if err := row.Scan(&pt.ID, &pt.CycleID, &pt.GamePlayerID, &situation, &meme, &pt.Status, &createdAt); err != nil {
		return round.PreparedTurn{}, err
	}
	pt.SituationText = situation.String
	pt.OriginalMemeID = meme.String
	pt.CreatedAt = parseTime(createdAt)
	return pt, nil
}

// --- Rounds ---

func (q queries) CreateRound(ctx context.Context, r round.Round) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO rounds (id, game_id, cycle_id, active_game_player_id, phase, situation_text, original_meme_id, status, started_at, deadline_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.GameID, r.CycleID, r.ActiveGamePlayerID, r.Phase, r.SituationText, r.OriginalMemeID, r.Status,
		r.StartedAt.UTC().Format(timeFmt), nullableTime(r.DeadlineAt), nullableTime(r.FinishedAt))
	return err
}

func (q queries) GetRound(ctx context.Context, id string) (round.Round, error) {
	return q.scanRound(q.q.QueryRowContext(ctx, `
		SELECT id, game_id, cycle_id, active_game_player_id, phase, situation_text, original_meme_id, status, started_at, deadline_at, finished_at
		FROM rounds WHERE id = ?`, id))
}

func (q queries) ListRoundsByGame(ctx context.Context, gameID string) ([]round.Round, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, game_id, cycle_id, active_game_player_id, phase, situation_text, original_meme_id, status, started_at, deadline_at, finished_at
		FROM rounds WHERE game_id = ?`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.Round
	for rows.Next() {
		r, err := q.scanRoundRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q queries) UpdateRound(ctx context.Context, r round.Round) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE rounds SET game_id = ?, cycle_id = ?, active_game_player_id = ?, phase = ?, situation_text = ?,
			original_meme_id = ?, status = ?, started_at = ?, deadline_at = ?, finished_at = ?
		WHERE id = ?`,
		r.GameID, r.CycleID, r.ActiveGamePlayerID, r.Phase, r.SituationText, r.OriginalMemeID, r.Status,
		r.StartedAt.UTC().Format(timeFmt), nullableTime(r.DeadlineAt), nullableTime(r.FinishedAt), r.ID)
	return err
}

func (q queries) scanRound(row *sql.Row) (round.Round, error) {
	return scanRound(row)
}

func (q queries) scanRoundRows(row rowScanner) (round.Round, error) {
	return scanRound(row)
}

func scanRound(row rowScanner) (round.Round, error) {
	var r round.Round
	var startedAt string
	var deadline, finished sql.NullString
	if err := row.Scan(&r.ID, &r.GameID, &r.CycleID, &r.ActiveGamePlayerID, &r.Phase, &r.SituationText,
		&r.OriginalMemeID, &r.Status, &startedAt, &deadline, &finished); err != nil {
		return round.Round{}, err
	}
	r.StartedAt = parseTime(startedAt)
	r.DeadlineAt = parseNullableTime(deadline)
	r.FinishedAt = parseNullableTime(finished)
	return r, nil
}

// --- Dealt hands ---

func (q queries) AddDealtHands(ctx context.Context, cycleID, gamePlayerID string, memeIDs []string, kind, roundID string) error {
	for _, memeID := range memeIDs {
		if _, err := q.q.ExecContext(ctx, `
			INSERT INTO dealt_hands (id, cycle_id, game_player_id, meme_id, kind, round_id)
			VALUES (?, ?, ?, ?, ?, ?)`,
			newID(), cycleID, gamePlayerID, memeID, kind, nullableString(roundID)); err != nil {
			return err
		}
	}
	return nil
}

func (q queries) ListDealtHands(ctx context.Context, cycleID string) ([]repository.DealtHand, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, cycle_id, game_player_id, meme_id, kind, round_id FROM dealt_hands WHERE cycle_id = ?`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.DealtHand
	for rows.Next() {
		var h repository.DealtHand
		var roundID sql.NullString
		if err := rows.Scan(&h.ID, &h.CycleID, &h.GamePlayerID, &h.MemeID, &h.Kind, &roundID); err != nil {
			return nil, err
		}
		h.RoundID = roundID.String
		out = append(out, h)
	}
	return out, rows.Err()
}
