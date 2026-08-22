-- +goose Up
-- The original UNIQUE constraint on dealt_hands omitted round_id, which
-- incorrectly forbids a player receiving the same meme in different rounds of
-- the same cycle. Recreate the table with round_id included.
CREATE TABLE dealt_hands_new (
    id             TEXT PRIMARY KEY,                      -- UUID
    cycle_id       TEXT NOT NULL REFERENCES game_cycles(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    meme_id        TEXT NOT NULL REFERENCES memes(id),
    kind           TEXT NOT NULL,                         -- PREPARATION | ROUND
    round_id       TEXT REFERENCES rounds(id),
    UNIQUE (cycle_id, game_player_id, meme_id, kind, round_id)
);

INSERT INTO dealt_hands_new (id, cycle_id, game_player_id, meme_id, kind, round_id)
    SELECT id, cycle_id, game_player_id, meme_id, kind, round_id FROM dealt_hands;

DROP TABLE dealt_hands;
ALTER TABLE dealt_hands_new RENAME TO dealt_hands;

-- +goose Down
CREATE TABLE dealt_hands_old (
    id             TEXT PRIMARY KEY,                      -- UUID
    cycle_id       TEXT NOT NULL REFERENCES game_cycles(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    meme_id        TEXT NOT NULL REFERENCES memes(id),
    kind           TEXT NOT NULL,                         -- PREPARATION | ROUND
    round_id       TEXT REFERENCES rounds(id),
    UNIQUE (cycle_id, game_player_id, meme_id, kind)
);

INSERT INTO dealt_hands_old (id, cycle_id, game_player_id, meme_id, kind, round_id)
    SELECT id, cycle_id, game_player_id, meme_id, kind, round_id FROM dealt_hands;

DROP TABLE dealt_hands;
ALTER TABLE dealt_hands_old RENAME TO dealt_hands;