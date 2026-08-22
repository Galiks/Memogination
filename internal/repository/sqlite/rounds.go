package sqlite

import (
	"context"
	"database/sql"

	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/repository"
)

// --- Round submissions ---

func (q queries) CreateRoundSubmission(ctx context.Context, s round.RoundSubmission) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO round_submissions (id, round_id, game_player_id, meme_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.RoundID, s.GamePlayerID, s.MemeID, s.CreatedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) GetRoundSubmission(ctx context.Context, id string) (round.RoundSubmission, error) {
	return q.scanRoundSubmission(q.q.QueryRowContext(ctx, `
		SELECT id, round_id, game_player_id, meme_id, created_at FROM round_submissions WHERE id = ?`, id))
}

func (q queries) ListRoundSubmissions(ctx context.Context, roundID string) ([]round.RoundSubmission, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, round_id, game_player_id, meme_id, created_at FROM round_submissions WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.RoundSubmission
	for rows.Next() {
		s, err := q.scanRoundSubmissionRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q queries) scanRoundSubmission(row *sql.Row) (round.RoundSubmission, error) {
	return scanRoundSubmission(row)
}

func (q queries) scanRoundSubmissionRows(row rowScanner) (round.RoundSubmission, error) {
	return scanRoundSubmission(row)
}

func scanRoundSubmission(row rowScanner) (round.RoundSubmission, error) {
	var s round.RoundSubmission
	var createdAt string
	if err := row.Scan(&s.ID, &s.RoundID, &s.GamePlayerID, &s.MemeID, &createdAt); err != nil {
		return round.RoundSubmission{}, err
	}
	s.CreatedAt = parseTime(createdAt)
	return s, nil
}

// --- Vote options ---

func (q queries) CreateVoteOption(ctx context.Context, vo round.VoteOption) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO vote_options (id, round_id, number, meme_id, owner_game_player_id, is_original)
		VALUES (?, ?, ?, ?, ?, ?)`,
		vo.ID, vo.RoundID, vo.Number, vo.MemeID, nullableString(vo.OwnerGamePlayerID), boolInt(vo.IsOriginal))
	return err
}

func (q queries) ListVoteOptions(ctx context.Context, roundID string) ([]round.VoteOption, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, round_id, number, meme_id, owner_game_player_id, is_original FROM vote_options WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.VoteOption
	for rows.Next() {
		var vo round.VoteOption
		var owner sql.NullString
		var isOriginal int
		if err := rows.Scan(&vo.ID, &vo.RoundID, &vo.Number, &vo.MemeID, &owner, &isOriginal); err != nil {
			return nil, err
		}
		vo.OwnerGamePlayerID = owner.String
		vo.IsOriginal = isOriginal != 0
		out = append(out, vo)
	}
	return out, rows.Err()
}

// --- Votes ---

func (q queries) CreateVote(ctx context.Context, v round.Vote) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO votes (id, round_id, game_player_id, vote_option_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		v.ID, v.RoundID, v.GamePlayerID, v.VoteOptionID, v.CreatedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) ListVotes(ctx context.Context, roundID string) ([]round.Vote, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, round_id, game_player_id, vote_option_id, created_at FROM votes WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.Vote
	for rows.Next() {
		var v round.Vote
		var createdAt string
		if err := rows.Scan(&v.ID, &v.RoundID, &v.GamePlayerID, &v.VoteOptionID, &createdAt); err != nil {
			return nil, err
		}
		v.CreatedAt = parseTime(createdAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

// --- Round scores ---

func (q queries) CreateRoundScore(ctx context.Context, rs round.RoundScore) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO round_scores (id, round_id, game_player_id, previous_score, delta, new_score)
		VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), rs.RoundID, rs.GamePlayerID, rs.PreviousScore, rs.Delta, rs.NewScore)
	return err
}

func (q queries) ListRoundScores(ctx context.Context, roundID string) ([]round.RoundScore, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT round_id, game_player_id, previous_score, delta, new_score FROM round_scores WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []round.RoundScore
	for rows.Next() {
		var rs round.RoundScore
		if err := rows.Scan(&rs.RoundID, &rs.GamePlayerID, &rs.PreviousScore, &rs.Delta, &rs.NewScore); err != nil {
			return nil, err
		}
		out = append(out, rs)
	}
	return out, rows.Err()
}

// --- Processed commands ---

func (q queries) CreateProcessedCommand(ctx context.Context, pc repository.ProcessedCommand) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO processed_commands (command_id, room_id, player_id, command_type, result_revision, processed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		pc.CommandID, pc.RoomID, nullableString(pc.PlayerID), pc.CommandType, pc.ResultRevision,
		pc.ProcessedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) IsProcessedCommand(ctx context.Context, commandID string) (bool, error) {
	var n int
	err := q.q.QueryRowContext(ctx, `SELECT count(*) FROM processed_commands WHERE command_id = ?`, commandID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// --- Game child-row deletion (used by SaveAggregate) ---

// DeleteDealtHandsByGame removes all dealt hands of a game's cycles.
func (q queries) DeleteDealtHandsByGame(ctx context.Context, gameID string) error {
	_, err := q.q.ExecContext(ctx, `
		DELETE FROM dealt_hands WHERE cycle_id IN (SELECT id FROM game_cycles WHERE game_id = ?)`, gameID)
	return err
}

// DeleteRoundsByGame removes all rounds of a game and their child rows
// (round_scores, votes, vote_options, round_submissions).
func (q queries) DeleteRoundsByGame(ctx context.Context, gameID string) error {
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM round_scores WHERE round_id IN (SELECT id FROM rounds WHERE game_id = ?)`, gameID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM votes WHERE round_id IN (SELECT id FROM rounds WHERE game_id = ?)`, gameID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM vote_options WHERE round_id IN (SELECT id FROM rounds WHERE game_id = ?)`, gameID); err != nil {
		return err
	}
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM round_submissions WHERE round_id IN (SELECT id FROM rounds WHERE game_id = ?)`, gameID); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `DELETE FROM rounds WHERE game_id = ?`, gameID)
	return err
}

// DeleteGameCyclesByGame removes all cycles of a game and their prepared turns.
func (q queries) DeleteGameCyclesByGame(ctx context.Context, gameID string) error {
	if _, err := q.q.ExecContext(ctx, `
		DELETE FROM prepared_turns WHERE cycle_id IN (SELECT id FROM game_cycles WHERE game_id = ?)`, gameID); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `DELETE FROM game_cycles WHERE game_id = ?`, gameID)
	return err
}

// DeleteGamePlayersByGame removes all game players of a game.
func (q queries) DeleteGamePlayersByGame(ctx context.Context, gameID string) error {
	_, err := q.q.ExecContext(ctx, `DELETE FROM game_players WHERE game_id = ?`, gameID)
	return err
}
