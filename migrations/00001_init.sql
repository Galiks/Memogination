-- +goose Up
CREATE TABLE rooms (
    id         TEXT PRIMARY KEY,                          -- UUID
    code       TEXT NOT NULL UNIQUE,                      -- short join code
    revision   INTEGER NOT NULL DEFAULT 0,                -- optimistic concurrency
    state      TEXT NOT NULL DEFAULT 'LOBBY',             -- LOBBY | IN_GAME | CLOSED
    created_at TEXT NOT NULL,                             -- ISO8601 UTC
    closed_at  TEXT                                       -- ISO8601 UTC
);

CREATE TABLE players (
    id         TEXT PRIMARY KEY,                          -- UUID
    room_id    TEXT NOT NULL REFERENCES rooms(id),
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'PLAYER',            -- HOST | PLAYER
    connected  INTEGER NOT NULL DEFAULT 0,
    joined_at  TEXT NOT NULL,                             -- ISO8601 UTC
    left_at    TEXT,                                      -- ISO8601 UTC
    UNIQUE (room_id, name COLLATE NOCASE)
);

CREATE TABLE player_sessions (
    id           TEXT PRIMARY KEY,                        -- UUID
    player_id    TEXT NOT NULL REFERENCES players(id),
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,                           -- ISO8601 UTC
    last_seen_at TEXT,                                    -- ISO8601 UTC
    revoked_at   TEXT                                     -- ISO8601 UTC
);

CREATE TABLE room_settings (
    room_id                       TEXT PRIMARY KEY REFERENCES rooms(id),
    min_players                   INTEGER NOT NULL DEFAULT 2,
    max_players                   INTEGER NOT NULL DEFAULT 10,
    hand_size                     INTEGER NOT NULL DEFAULT 5,
    preparation_timeout_seconds   INTEGER NOT NULL DEFAULT 0,
    round_selection_timeout_seconds INTEGER NOT NULL DEFAULT 0,
    voting_timeout_seconds        INTEGER NOT NULL DEFAULT 0,
    infinite_game                 INTEGER NOT NULL DEFAULT 0,
    situation_separator           TEXT NOT NULL DEFAULT '*',
    score_config                  TEXT NOT NULL            -- JSON
);

CREATE TABLE games (
    id                TEXT PRIMARY KEY,                   -- UUID
    room_id           TEXT NOT NULL REFERENCES rooms(id),
    state             TEXT NOT NULL DEFAULT 'ACTIVE',     -- ACTIVE | FINISHED
    revision          INTEGER NOT NULL DEFAULT 0,
    settings_snapshot TEXT NOT NULL,                      -- JSON
    current_cycle_id  TEXT,
    current_round_id  TEXT,
    started_at        TEXT NOT NULL,                      -- ISO8601 UTC
    finished_at       TEXT                                -- ISO8601 UTC
);

CREATE TABLE game_players (
    id                   TEXT PRIMARY KEY,                -- UUID
    game_id              TEXT NOT NULL REFERENCES games(id),
    player_id            TEXT NOT NULL REFERENCES players(id),
    display_name         TEXT NOT NULL,
    turn_order           INTEGER NOT NULL,
    score                INTEGER NOT NULL DEFAULT 0,
    participation_status TEXT NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE | LEFT | KICKED | SKIPPED
    UNIQUE (game_id, player_id)
);

CREATE TABLE game_cycles (
    id          TEXT PRIMARY KEY,                         -- UUID
    game_id     TEXT NOT NULL REFERENCES games(id),
    number      INTEGER NOT NULL,
    started_at  TEXT NOT NULL,                            -- ISO8601 UTC
    finished_at TEXT,                                     -- ISO8601 UTC
    UNIQUE (game_id, number)
);

CREATE TABLE prepared_turns (
    id               TEXT PRIMARY KEY,                    -- UUID
    cycle_id         TEXT NOT NULL REFERENCES game_cycles(id),
    game_player_id   TEXT NOT NULL REFERENCES game_players(id),
    situation_text   TEXT,
    original_meme_id TEXT REFERENCES memes(id),
    status           TEXT NOT NULL DEFAULT 'PENDING',     -- PENDING | READY
    created_at       TEXT NOT NULL,                       -- ISO8601 UTC
    UNIQUE (cycle_id, game_player_id)
);

CREATE TABLE rounds (
    id                   TEXT PRIMARY KEY,                -- UUID
    game_id              TEXT NOT NULL REFERENCES games(id),
    cycle_id             TEXT NOT NULL REFERENCES game_cycles(id),
    active_game_player_id TEXT NOT NULL REFERENCES game_players(id),
    phase                TEXT NOT NULL DEFAULT 'ROUND_SELECTION', -- ROUND_SELECTION | ROUND_VOTING | ROUND_RESULTS
    situation_text       TEXT NOT NULL,
    original_meme_id     TEXT NOT NULL REFERENCES memes(id),
    status               TEXT NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE | FINISHED | CANCELLED
    started_at           TEXT NOT NULL,                   -- ISO8601 UTC
    deadline_at          TEXT,                            -- ISO8601 UTC
    finished_at          TEXT                             -- ISO8601 UTC
);

CREATE TABLE dealt_hands (
    id             TEXT PRIMARY KEY,                      -- UUID
    cycle_id       TEXT NOT NULL REFERENCES game_cycles(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    meme_id        TEXT NOT NULL REFERENCES memes(id),
    kind           TEXT NOT NULL,                         -- PREPARATION | ROUND
    round_id       TEXT REFERENCES rounds(id),
    UNIQUE (cycle_id, game_player_id, meme_id, kind)
);

CREATE TABLE round_submissions (
    id           TEXT PRIMARY KEY,                        -- UUID
    round_id     TEXT NOT NULL REFERENCES rounds(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    meme_id      TEXT NOT NULL REFERENCES memes(id),
    created_at   TEXT NOT NULL,                           -- ISO8601 UTC
    UNIQUE (round_id, game_player_id)
);

CREATE TABLE vote_options (
    id                   TEXT PRIMARY KEY,                -- UUID
    round_id             TEXT NOT NULL REFERENCES rounds(id),
    number               INTEGER NOT NULL,
    meme_id              TEXT NOT NULL REFERENCES memes(id),
    owner_game_player_id TEXT REFERENCES game_players(id),
    is_original          INTEGER NOT NULL DEFAULT 0,
    UNIQUE (round_id, number)
);

CREATE TABLE votes (
    id             TEXT PRIMARY KEY,                      -- UUID
    round_id       TEXT NOT NULL REFERENCES rounds(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    vote_option_id TEXT NOT NULL REFERENCES vote_options(id),
    created_at     TEXT NOT NULL,                         -- ISO8601 UTC
    UNIQUE (round_id, game_player_id)
);

CREATE TABLE round_scores (
    id             TEXT PRIMARY KEY,                      -- UUID
    round_id       TEXT NOT NULL REFERENCES rounds(id),
    game_player_id TEXT NOT NULL REFERENCES game_players(id),
    previous_score INTEGER NOT NULL,
    delta          INTEGER NOT NULL,
    new_score      INTEGER NOT NULL,
    UNIQUE (round_id, game_player_id)
);

CREATE TABLE processed_commands (
    command_id      TEXT PRIMARY KEY,
    room_id         TEXT NOT NULL REFERENCES rooms(id),
    player_id       TEXT,
    command_type    TEXT NOT NULL,
    result_revision INTEGER NOT NULL,
    processed_at    TEXT NOT NULL                         -- ISO8601 UTC
);

CREATE TABLE memes (
    id                TEXT PRIMARY KEY,                   -- UUID
    original_path     TEXT NOT NULL,
    screen_path       TEXT NOT NULL,
    thumbnail_path    TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type         TEXT NOT NULL,
    sha256            TEXT NOT NULL UNIQUE,
    enabled           INTEGER NOT NULL DEFAULT 1,
    source            TEXT NOT NULL DEFAULT 'upload',
    created_at        TEXT NOT NULL                       -- ISO8601 UTC
);

CREATE TABLE situations (
    id         TEXT PRIMARY KEY,                          -- UUID
    text       TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    source     TEXT NOT NULL DEFAULT 'manual',
    created_at TEXT NOT NULL                              -- ISO8601 UTC
);

-- +goose Down
DROP TABLE IF EXISTS situations;
DROP TABLE IF EXISTS memes;
DROP TABLE IF EXISTS processed_commands;
DROP TABLE IF EXISTS round_scores;
DROP TABLE IF EXISTS votes;
DROP TABLE IF EXISTS vote_options;
DROP TABLE IF EXISTS round_submissions;
DROP TABLE IF EXISTS dealt_hands;
DROP TABLE IF EXISTS rounds;
DROP TABLE IF EXISTS prepared_turns;
DROP TABLE IF EXISTS game_cycles;
DROP TABLE IF EXISTS game_players;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS room_settings;
DROP TABLE IF EXISTS player_sessions;
DROP TABLE IF EXISTS players;
DROP TABLE IF EXISTS rooms;