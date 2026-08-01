-- V2-M1 separates the film people browse from the files they can play.
--
-- The legacy file columns remain on movies for one compatibility cycle. The
-- current frontend and the old stream routes still read them; the application
-- mirrors the primary movie_file into those columns. New code treats
-- movie_files as the source of truth.

CREATE TABLE movie_files (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id     INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,

    path         TEXT    NOT NULL UNIQUE,
    file_name    TEXT    NOT NULL,
    size_bytes   INTEGER NOT NULL,
    modified_at  INTEGER NOT NULL,

    first_seen_scan INTEGER NOT NULL,
    last_seen_scan  INTEGER NOT NULL,
    is_primary      INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),

    -- Inspection is deliberately lazy. Merely asking how a file would play
    -- must not download ffmpeg. An explicit inspection or a remux fills these
    -- columns and the audio-track table below.
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

CREATE INDEX idx_movie_files_movie ON movie_files (movie_id, id);
CREATE INDEX idx_movie_files_last_seen ON movie_files (last_seen_scan);
CREATE INDEX idx_movies_year_title ON movies (year, title COLLATE NOCASE);
CREATE UNIQUE INDEX idx_movie_files_one_primary
    ON movie_files (movie_id) WHERE is_primary = 1;

CREATE TABLE movie_file_audio_tracks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_file_id INTEGER NOT NULL REFERENCES movie_files(id) ON DELETE CASCADE,
    stream_index  INTEGER NOT NULL,
    codec         TEXT    NOT NULL,
    language      TEXT,
    title         TEXT,
    channels      TEXT,
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),

    UNIQUE (movie_file_id, stream_index)
);

CREATE INDEX idx_movie_file_audio_tracks_file
    ON movie_file_audio_tracks (movie_file_id, stream_index);

-- Every existing movie row represented exactly one file. Copy it without
-- changing the movie id, metadata or progress. Consolidation happens in Go,
-- where conflicting TMDB ids and progress can be handled conservatively.
INSERT INTO movie_files (
    movie_id, path, file_name, size_bytes, modified_at,
    first_seen_scan, last_seen_scan, is_primary,
    media_status, media_duration_seconds
)
SELECT
    id, path, file_name, size_bytes, modified_at,
    first_seen_scan, last_seen_scan, 1,
    'pending', duration_seconds
FROM movies;
