-- Playback position, kept on the movie row.
--
-- A separate table would be the shape for several viewers, but decision 6 rules
-- multi-user out of v1 and there is exactly one person watching. One row per
-- film, one position on it.

-- Seconds, from the ffmpeg probe when a file is first rewrapped, or from TMDB's
-- runtime when that is all we have. Null means unknown, which the interface has
-- to cope with: a remux stream is a pipe and carries no duration of its own.
ALTER TABLE movies ADD COLUMN duration_seconds REAL;

-- Where the viewer stopped. Zero means "not started", which is not the same as
-- "watched the first second and left".
ALTER TABLE movies ADD COLUMN position_seconds REAL NOT NULL DEFAULT 0;

-- Unix seconds of the last progress report. Orders the continue-watching row,
-- and zero keeps a film out of it entirely.
ALTER TABLE movies ADD COLUMN watched_at INTEGER NOT NULL DEFAULT 0;

-- Recomputed on every progress report rather than set once: a film watched to
-- the end leaves the continue-watching row, and starting it again brings it
-- back. See finishedRule in internal/library/progress.go.
ALTER TABLE movies ADD COLUMN finished INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_movies_continue_watching ON movies (finished, watched_at);
