-- +goose Up
ALTER TABLE games ADD COLUMN phase TEXT;
ALTER TABLE games ADD COLUMN phase_deadline_at TEXT;

-- +goose Down
ALTER TABLE games DROP COLUMN phase;
ALTER TABLE games DROP COLUMN phase_deadline_at;