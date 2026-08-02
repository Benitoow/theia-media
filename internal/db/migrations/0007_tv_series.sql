-- V2-M3 adds TV series without turning the stable film model into a polymorphic
-- tree. A local playable item represents one episode or an ordered group such
-- as S01E01E02; one or more physical files can sit below it.

CREATE TABLE series (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    year       INTEGER,

    tmdb_id        INTEGER,
    tmdb_name      TEXT,
    original_name  TEXT,
    overview       TEXT,
    first_air_date TEXT,
    poster_path    TEXT,
    backdrop_path  TEXT,
    vote_average   REAL,
    genres_json    TEXT,
    cast_json      TEXT,
    creators_json  TEXT,

    metadata_status     TEXT    NOT NULL DEFAULT 'pending'
                        CHECK (metadata_status IN ('pending', 'ok', 'not_found', 'error')),
    metadata_fetched_at INTEGER NOT NULL DEFAULT 0,
    added_at            INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE INDEX idx_series_title ON series (title COLLATE NOCASE, year);
CREATE INDEX idx_series_metadata_freshness
    ON series (metadata_status, metadata_fetched_at);
CREATE INDEX idx_series_tmdb_id
    ON series (tmdb_id) WHERE tmdb_id IS NOT NULL;

CREATE TABLE seasons (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id     INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL CHECK (season_number >= 0),

    tmdb_id       INTEGER,
    name          TEXT,
    overview      TEXT,
    air_date      TEXT,
    poster_path   TEXT,
    episode_count INTEGER,

    metadata_status     TEXT    NOT NULL DEFAULT 'pending'
                        CHECK (metadata_status IN ('pending', 'ok', 'not_found', 'error')),
    metadata_fetched_at INTEGER NOT NULL DEFAULT 0,
    added_at            INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,

    UNIQUE (series_id, season_number)
);

CREATE INDEX idx_seasons_series ON seasons (series_id, season_number);
CREATE INDEX idx_seasons_tmdb_id
    ON seasons (tmdb_id) WHERE tmdb_id IS NOT NULL;

CREATE TABLE episodes (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id      INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    episode_number INTEGER NOT NULL CHECK (episode_number >= 0),
    local_title    TEXT,

    tmdb_id         INTEGER,
    name            TEXT,
    overview        TEXT,
    air_date        TEXT,
    still_path      TEXT,
    runtime_minutes INTEGER,
    vote_average    REAL,
    metadata_status TEXT    NOT NULL DEFAULT 'pending'
                    CHECK (metadata_status IN ('pending', 'ok', 'not_found', 'error')),
    metadata_fetched_at INTEGER NOT NULL DEFAULT 0,

    UNIQUE (season_id, episode_number)
);

CREATE INDEX idx_episodes_season ON episodes (season_id, episode_number);
CREATE INDEX idx_episodes_tmdb_id
    ON episodes (tmdb_id) WHERE tmdb_id IS NOT NULL;

CREATE TABLE episode_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id    INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    episode_key  TEXT    NOT NULL,
    first_episode INTEGER NOT NULL CHECK (first_episode >= 0),
    last_episode  INTEGER NOT NULL CHECK (last_episode >= first_episode),

    duration_seconds REAL,
    position_seconds REAL    NOT NULL DEFAULT 0,
    watched_at       INTEGER NOT NULL DEFAULT 0,
    finished         INTEGER NOT NULL DEFAULT 0 CHECK (finished IN (0, 1)),
    added_at         INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,

    UNIQUE (season_id, episode_key)
);

CREATE INDEX idx_episode_items_season
    ON episode_items (season_id, first_episode, id);
CREATE INDEX idx_episode_items_continue
    ON episode_items (finished, watched_at);

CREATE TABLE episode_item_members (
    episode_item_id INTEGER NOT NULL REFERENCES episode_items(id) ON DELETE CASCADE,
    episode_id      INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    ordinal         INTEGER NOT NULL CHECK (ordinal >= 0),

    PRIMARY KEY (episode_item_id, episode_id),
    UNIQUE (episode_item_id, ordinal)
);

CREATE INDEX idx_episode_item_members_episode
    ON episode_item_members (episode_id, episode_item_id);

CREATE TABLE episode_files (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_item_id INTEGER NOT NULL REFERENCES episode_items(id) ON DELETE CASCADE,

    path         TEXT    NOT NULL UNIQUE,
    file_name    TEXT    NOT NULL,
    size_bytes   INTEGER NOT NULL,
    modified_at  INTEGER NOT NULL,

    first_seen_scan INTEGER NOT NULL,
    last_seen_scan  INTEGER NOT NULL,
    is_primary      INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),

    media_status           TEXT NOT NULL DEFAULT 'pending'
                           CHECK (media_status IN ('pending', 'ok', 'error')),
    media_container        TEXT,
    media_duration_seconds REAL,
    video_stream_index     INTEGER,
    video_codec            TEXT,
    video_width            INTEGER,
    video_height           INTEGER,
    media_inspected_at     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_episode_files_item ON episode_files (episode_item_id, id);
CREATE INDEX idx_episode_files_last_seen ON episode_files (last_seen_scan);
CREATE UNIQUE INDEX idx_episode_files_one_primary
    ON episode_files (episode_item_id) WHERE is_primary = 1;

CREATE TABLE episode_file_audio_tracks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_file_id INTEGER NOT NULL REFERENCES episode_files(id) ON DELETE CASCADE,
    stream_index    INTEGER NOT NULL,
    codec           TEXT    NOT NULL,
    language        TEXT,
    title           TEXT,
    channels        TEXT,
    is_default      INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),

    UNIQUE (episode_file_id, stream_index)
);

CREATE INDEX idx_episode_file_audio_tracks_file
    ON episode_file_audio_tracks (episode_file_id, stream_index);
