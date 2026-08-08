-- Subtitle tracks, finally built. Decision 3 always allowed them: text tracks
-- inside the container plus `.srt` files next to the media, with image formats
-- (PGS, VobSub) out because showing one means burning it into the picture.
--
-- Both kinds live in one table. An embedded track carries its ffmpeg stream
-- index; an external file carries a path. Exactly one of the two is set, which
-- the CHECK enforces rather than leaving to the code that writes here.
--
-- source_path never crosses the HTTP boundary, exactly like movie_files.path:
-- the browser selects a row id and the server resolves the file itself.

CREATE TABLE movie_file_subtitle_tracks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_file_id INTEGER NOT NULL REFERENCES movie_files(id) ON DELETE CASCADE,

    stream_index  INTEGER,
    source_path   TEXT,

    codec         TEXT    NOT NULL,
    language      TEXT,
    title         TEXT,
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    is_forced     INTEGER NOT NULL DEFAULT 0 CHECK (is_forced IN (0, 1)),

    CHECK ((stream_index IS NULL) <> (source_path IS NULL))
);

CREATE UNIQUE INDEX idx_movie_subtitles_embedded
    ON movie_file_subtitle_tracks (movie_file_id, stream_index)
    WHERE stream_index IS NOT NULL;
CREATE UNIQUE INDEX idx_movie_subtitles_external
    ON movie_file_subtitle_tracks (movie_file_id, source_path)
    WHERE source_path IS NOT NULL;

CREATE TABLE episode_file_subtitle_tracks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_file_id INTEGER NOT NULL REFERENCES episode_files(id) ON DELETE CASCADE,

    stream_index    INTEGER,
    source_path     TEXT,

    codec           TEXT    NOT NULL,
    language        TEXT,
    title           TEXT,
    is_default      INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    is_forced       INTEGER NOT NULL DEFAULT 0 CHECK (is_forced IN (0, 1)),

    CHECK ((stream_index IS NULL) <> (source_path IS NULL))
);

CREATE UNIQUE INDEX idx_episode_subtitles_embedded
    ON episode_file_subtitle_tracks (episode_file_id, stream_index)
    WHERE stream_index IS NOT NULL;
CREATE UNIQUE INDEX idx_episode_subtitles_external
    ON episode_file_subtitle_tracks (episode_file_id, source_path)
    WHERE source_path IS NOT NULL;

-- Every file inspected before this migration has media_status = 'ok' and no
-- subtitle rows, which is indistinguishable from a file that genuinely has
-- none. This flag tells the two apart.
--
-- The alternative was resetting 274 films to 'pending' and re-probing the lot,
-- which would have thrown away durations and resolutions already measured to
-- learn one new fact. Instead the next playback of a file re-probes it once --
-- it was about to run ffmpeg anyway -- and the flag makes sure it is once.
ALTER TABLE movie_files   ADD COLUMN subtitles_scanned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE episode_files ADD COLUMN subtitles_scanned INTEGER NOT NULL DEFAULT 0;
