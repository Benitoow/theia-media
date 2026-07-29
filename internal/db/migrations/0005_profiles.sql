-- Cosmetic household profiles.
--
-- A profile is not an account: it has no credential, permission or session.
-- It is only the key that separates playback progress for people sharing one
-- library. The API deliberately lets every client list and select every
-- profile.

CREATE TABLE profiles (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT,
    is_default        INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    avatar_data       BLOB,
    avatar_media_type TEXT,
    avatar_version    INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- There is always one compatibility fallback for clients that predate
-- profiles and therefore send no profile header. Its NULL name is intentional:
-- the frontend can call it "Principal" or "Default" without putting a
-- user-facing language in the database.
CREATE UNIQUE INDEX idx_profiles_one_default
    ON profiles (is_default)
    WHERE is_default = 1;

INSERT INTO profiles (id, name, is_default, created_at, updated_at)
VALUES (1, NULL, 1, unixepoch(), unixepoch());

CREATE TABLE playback_progress (
    profile_id       INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    movie_id         INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    position_seconds REAL    NOT NULL DEFAULT 0,
    watched_at       INTEGER NOT NULL DEFAULT 0,
    finished         INTEGER NOT NULL DEFAULT 0 CHECK (finished IN (0, 1)),
    PRIMARY KEY (profile_id, movie_id)
);

-- Preserve every remembered state from the single-viewer schema. Duration is
-- not copied: it describes the media file and remains on movies.
INSERT INTO playback_progress
    (profile_id, movie_id, position_seconds, watched_at, finished)
SELECT
    1, id, position_seconds, watched_at, finished
FROM movies
WHERE position_seconds > 0 OR watched_at > 0 OR finished = 1;

CREATE INDEX idx_playback_progress_continue
    ON playback_progress (profile_id, finished, watched_at);

-- Keep the three legacy movie columns for one release cycle. The updater keeps
-- the previous executable beside the new one and may roll back to it; removing
-- columns that v1.2 still selects would turn that safety net into a crash.
--
-- If an old binary writes progress after a rollback, mirror it into the
-- default profile so a later re-upgrade does not lose what was watched in
-- between. New code also writes these columns for the default profile, so the
-- old binary can read progress recorded before a rollback.
CREATE TRIGGER sync_legacy_progress_to_default
AFTER UPDATE OF position_seconds, watched_at, finished ON movies
BEGIN
    INSERT INTO playback_progress
        (profile_id, movie_id, position_seconds, watched_at, finished)
    VALUES (
        (SELECT id FROM profiles WHERE is_default = 1 LIMIT 1),
        NEW.id,
        NEW.position_seconds,
        NEW.watched_at,
        NEW.finished
    )
    ON CONFLICT(profile_id, movie_id) DO UPDATE SET
        position_seconds = excluded.position_seconds,
        watched_at       = excluded.watched_at,
        finished         = excluded.finished;
END;
