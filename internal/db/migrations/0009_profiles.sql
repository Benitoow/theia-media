-- Profiles, second attempt. Decision 48.
--
-- Nothing here is restored from the implementation removed in 0005: not the
-- table shape, not the avatar column, not the header that selected a viewer.
-- What survives from that round is the reasoning, and one hard-won detail --
-- a NULL name means "the default profile", so the interface can call it
-- "Profil principal", "Default profile" or a future locale without a word of
-- French ever being stored in SQLite (decision 25).

CREATE TABLE profiles (
    id             INTEGER PRIMARY KEY,
    name           TEXT CHECK (name IS NULL OR length(trim(name)) BETWEEN 1 AND 40),
    avatar_bytes   BLOB,
    avatar_type    TEXT,
    avatar_version INTEGER NOT NULL DEFAULT 0,
    created_at     INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Progress leaves the media row and becomes per viewer. Duration deliberately
-- does not: it describes the file, not the person watching it, and keeping it
-- on the media row is what lets an unwatched film still know how long it is.
CREATE TABLE movie_progress (
    profile_id       INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    movie_id         INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    position_seconds REAL    NOT NULL DEFAULT 0,
    watched_at       INTEGER NOT NULL DEFAULT 0,
    finished         INTEGER NOT NULL DEFAULT 0 CHECK (finished IN (0, 1)),
    PRIMARY KEY (profile_id, movie_id)
);

CREATE TABLE episode_progress (
    profile_id       INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    episode_item_id  INTEGER NOT NULL REFERENCES episode_items(id) ON DELETE CASCADE,
    position_seconds REAL    NOT NULL DEFAULT 0,
    watched_at       INTEGER NOT NULL DEFAULT 0,
    finished         INTEGER NOT NULL DEFAULT 0 CHECK (finished IN (0, 1)),
    PRIMARY KEY (profile_id, episode_item_id)
);

-- The continue-watching row is the only query that scans progress by itself.
CREATE INDEX movie_progress_continue
    ON movie_progress(profile_id, finished, watched_at DESC);

CREATE INDEX episode_progress_continue
    ON episode_progress(profile_id, finished, watched_at DESC);

-- The single viewer that exists today becomes the default profile, and keeps
-- every position. Both families are copied: films have carried progress since
-- 0001, episodes since 0007, and a migration that took only one of them would
-- silently discard a series history (decision 41).
INSERT INTO profiles (id, name) VALUES (1, NULL);

INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished)
SELECT 1, id, position_seconds, watched_at, finished
FROM movies
WHERE position_seconds > 0 OR watched_at > 0 OR finished = 1;

INSERT INTO episode_progress (profile_id, episode_item_id, position_seconds, watched_at, finished)
SELECT 1, id, position_seconds, watched_at, finished
FROM episode_items
WHERE position_seconds > 0 OR watched_at > 0 OR finished = 1;

-- movies.position_seconds, watched_at and finished are deliberately left in
-- place and kept in step with the default profile by the Go layer. The updater
-- keeps the previous executable as a rollback target, and v1.5.0 reads those
-- columns at startup; dropping them here would strand anyone who rolls back.
-- Episodes need no such mirror -- v1.5.0 has never heard of them.
