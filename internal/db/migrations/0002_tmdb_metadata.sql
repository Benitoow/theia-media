-- TMDB metadata, kept on the movie row rather than in a side table. The
-- relationship is one to one and the home screen reads all of it at once, so a
-- join would buy nothing.

ALTER TABLE movies ADD COLUMN tmdb_id        INTEGER;
ALTER TABLE movies ADD COLUMN tmdb_title     TEXT;
ALTER TABLE movies ADD COLUMN overview       TEXT;
ALTER TABLE movies ADD COLUMN release_date   TEXT;
ALTER TABLE movies ADD COLUMN poster_path    TEXT;
ALTER TABLE movies ADD COLUMN backdrop_path  TEXT;
ALTER TABLE movies ADD COLUMN runtime_minutes INTEGER;
ALTER TABLE movies ADD COLUMN vote_average   REAL;
ALTER TABLE movies ADD COLUMN director       TEXT;

-- JSON blobs. Neither is ever queried by content, only read back whole, so
-- normalising them into tables would be structure for its own sake.
ALTER TABLE movies ADD COLUMN genres_json    TEXT;
ALTER TABLE movies ADD COLUMN cast_json      TEXT;

-- 'pending'   never looked up
-- 'ok'        TMDB matched it
-- 'not_found' TMDB has no such film, or the parsed title was too mangled
-- 'error'     the lookup failed for a reason that is not the film's fault
ALTER TABLE movies ADD COLUMN metadata_status TEXT NOT NULL DEFAULT 'pending';

-- Unix seconds of the last completed lookup, successful or not. Zero means
-- never. Staleness is judged from this; see internal/library/metadata.go for
-- the two lifetimes and docs/DECISIONS.md for why they differ.
ALTER TABLE movies ADD COLUMN metadata_fetched_at INTEGER NOT NULL DEFAULT 0;

-- Finding the next films to enrich is the one hot query the scan runs
-- repeatedly, and it filters on exactly these two columns.
CREATE INDEX idx_movies_metadata_freshness ON movies (metadata_status, metadata_fetched_at);
