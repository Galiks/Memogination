package sqlite

import (
	"context"
	"database/sql"

	"github.com/memomarium/memomarium/internal/domain/content"
)

// --- Memes ---

func (q queries) CreateMeme(ctx context.Context, m content.Meme) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO memes (id, original_path, screen_path, thumbnail_path, original_filename, mime_type, sha256, enabled, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.OriginalPath, m.ScreenPath, m.ThumbnailPath, m.OriginalFilename, m.MimeType, m.SHA256,
		boolInt(m.Enabled), m.Source, m.CreatedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) GetMeme(ctx context.Context, id string) (content.Meme, error) {
	return q.scanMeme(q.q.QueryRowContext(ctx, `
		SELECT id, original_path, screen_path, thumbnail_path, original_filename, mime_type, sha256, enabled, source, created_at
		FROM memes WHERE id = ?`, id))
}

func (q queries) GetMemeBySHA256(ctx context.Context, sha256 string) (content.Meme, error) {
	return q.scanMeme(q.q.QueryRowContext(ctx, `
		SELECT id, original_path, screen_path, thumbnail_path, original_filename, mime_type, sha256, enabled, source, created_at
		FROM memes WHERE sha256 = ?`, sha256))
}

func (q queries) UpdateMeme(ctx context.Context, m content.Meme) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE memes SET original_path = ?, screen_path = ?, thumbnail_path = ?, original_filename = ?,
			mime_type = ?, sha256 = ?, enabled = ?, source = ?, created_at = ?
		WHERE id = ?`,
		m.OriginalPath, m.ScreenPath, m.ThumbnailPath, m.OriginalFilename, m.MimeType, m.SHA256,
		boolInt(m.Enabled), m.Source, m.CreatedAt.UTC().Format(timeFmt), m.ID)
	return err
}

func (q queries) DeleteMeme(ctx context.Context, id string) error {
	_, err := q.q.ExecContext(ctx, `DELETE FROM memes WHERE id = ?`, id)
	return err
}

func (q queries) ListMemes(ctx context.Context) ([]content.Meme, error) {
	rows, err := q.q.QueryContext(ctx, `
		SELECT id, original_path, screen_path, thumbnail_path, original_filename, mime_type, sha256, enabled, source, created_at
		FROM memes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []content.Meme{}
	for rows.Next() {
		m, err := q.scanMemeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (q queries) scanMeme(row *sql.Row) (content.Meme, error) {
	return scanMeme(row)
}

func (q queries) scanMemeRows(row rowScanner) (content.Meme, error) {
	return scanMeme(row)
}

func scanMeme(row rowScanner) (content.Meme, error) {
	var m content.Meme
	var enabled int
	var createdAt string
	if err := row.Scan(&m.ID, &m.OriginalPath, &m.ScreenPath, &m.ThumbnailPath, &m.OriginalFilename,
		&m.MimeType, &m.SHA256, &enabled, &m.Source, &createdAt); err != nil {
		return content.Meme{}, err
	}
	m.Enabled = enabled != 0
	m.CreatedAt = parseTime(createdAt)
	return m, nil
}

// --- Situations ---

func (q queries) CreateSituation(ctx context.Context, s content.Situation) error {
	_, err := q.q.ExecContext(ctx, `
		INSERT INTO situations (id, text, enabled, source, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.Text, boolInt(s.Enabled), s.Source, s.CreatedAt.UTC().Format(timeFmt))
	return err
}

func (q queries) GetSituation(ctx context.Context, id string) (content.Situation, error) {
	return q.scanSituation(q.q.QueryRowContext(ctx, `
		SELECT id, text, enabled, source, created_at FROM situations WHERE id = ?`, id))
}

func (q queries) UpdateSituation(ctx context.Context, s content.Situation) error {
	_, err := q.q.ExecContext(ctx, `
		UPDATE situations SET text = ?, enabled = ?, source = ?, created_at = ? WHERE id = ?`,
		s.Text, boolInt(s.Enabled), s.Source, s.CreatedAt.UTC().Format(timeFmt), s.ID)
	return err
}

func (q queries) DeleteSituation(ctx context.Context, id string) error {
	_, err := q.q.ExecContext(ctx, `DELETE FROM situations WHERE id = ?`, id)
	return err
}

func (q queries) ListSituations(ctx context.Context) ([]content.Situation, error) {
	rows, err := q.q.QueryContext(ctx, `SELECT id, text, enabled, source, created_at FROM situations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []content.Situation{}
	for rows.Next() {
		s, err := q.scanSituationRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q queries) scanSituation(row *sql.Row) (content.Situation, error) {
	return scanSituation(row)
}

func (q queries) scanSituationRows(row rowScanner) (content.Situation, error) {
	return scanSituation(row)
}

func scanSituation(row rowScanner) (content.Situation, error) {
	var s content.Situation
	var enabled int
	var createdAt string
	if err := row.Scan(&s.ID, &s.Text, &enabled, &s.Source, &createdAt); err != nil {
		return content.Situation{}, err
	}
	s.Enabled = enabled != 0
	s.CreatedAt = parseTime(createdAt)
	return s, nil
}
