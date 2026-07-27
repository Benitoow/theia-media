-- The library as of M1: files found on disk, with whatever the filename gave
-- away. Everything TMDB provides arrives in M2 and is deliberately absent here
-- rather than sitting empty.

-- A strictly increasing counter, bumped once at the start of every scan.
--
-- Reconciliation deliberately does not key off wall-clock time. Two scans
-- within the same second are indistinguishable by timestamp, which makes an
-- update look like an insert and stops a vanished file from ever being pruned;
-- an NTP correction between two scans breaks it more thoroughly still. A
-- counter has neither problem.
CREATE TABLE scan_generation (
    id    INTEGER PRIMARY KEY CHECK (id = 1),
    value INTEGER NOT NULL
);

INSERT INTO scan_generation (id, value) VALUES (1, 0);

CREATE TABLE movies (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Absolute path, and the natural identity of a row: finding the same path
    -- again is an update, not a second film.
    path         TEXT    NOT NULL UNIQUE,
    file_name    TEXT    NOT NULL,
    size_bytes   INTEGER NOT NULL,
    modified_at  INTEGER NOT NULL,

    -- Parsed from the filename. title is never empty -- the parser falls back
    -- to the raw filename rather than giving up -- but year is genuinely
    -- unknown for plenty of real files, so it is nullable and means it.
    title        TEXT    NOT NULL,
    year         INTEGER,

    -- The scan that first inserted this row, and the most recent one that saw
    -- the file. first_seen_scan equal to the running generation means the row
    -- was just created; last_seen_scan behind it means the file is gone.
    first_seen_scan INTEGER NOT NULL,
    last_seen_scan  INTEGER NOT NULL,

    -- Human-facing only. Nothing branches on these.
    added_at     INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX idx_movies_title ON movies (title);
CREATE INDEX idx_movies_last_seen_scan ON movies (last_seen_scan);
