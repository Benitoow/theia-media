-- Everything TMDB already sends and Theia used to throw away.
--
-- None of this costs a request: the tagline, the certificate, the collection and
-- the crew arrive in the same details call that was already being made, either
-- as plain fields or through append_to_response. The film page was showing a
-- tenth of the record it had paid for.
--
-- Columns rather than a side table, for the same reason as migration 0002: the
-- relationship is one to one and the pages read all of it at once.

ALTER TABLE movies ADD COLUMN tagline               TEXT;
ALTER TABLE movies ADD COLUMN original_title        TEXT;
ALTER TABLE movies ADD COLUMN original_language     TEXT;

-- The age rating as an authority wrote it, with the country that issued it.
-- "16" alone is not information: the interface says which board said so.
ALTER TABLE movies ADD COLUMN certification         TEXT;
ALTER TABLE movies ADD COLUMN certification_country TEXT;

-- The TMDB collection a film belongs to. The id is what makes the other parts
-- findable, and it is indexed for exactly that query; the name and the poster
-- are kept so the row can be drawn without a second lookup.
ALTER TABLE movies ADD COLUMN collection_id          INTEGER;
ALTER TABLE movies ADD COLUMN collection_name        TEXT;
ALTER TABLE movies ADD COLUMN collection_poster_path TEXT;

-- Crew beyond the director, as JSON: read back whole, never queried by content.
-- Each entry carries a role code, not TMDB's English job title (decision 25).
ALTER TABLE movies ADD COLUMN crew_json TEXT;

CREATE INDEX idx_movies_collection
    ON movies (collection_id) WHERE collection_id IS NOT NULL;

-- The same enrichment for series. `status` holds a code -- 'ended',
-- 'returning' -- rather than TMDB's English label, so the interface can say it
-- in the language the browser asked for.
ALTER TABLE series ADD COLUMN tagline               TEXT;
ALTER TABLE series ADD COLUMN last_air_date         TEXT;
ALTER TABLE series ADD COLUMN status                TEXT;
ALTER TABLE series ADD COLUMN certification         TEXT;
ALTER TABLE series ADD COLUMN certification_country TEXT;
ALTER TABLE series ADD COLUMN networks_json         TEXT;

-- The field set a row was written with.
--
-- Without this, every film already in the library keeps its 'ok' status and its
-- recent fetch timestamp, so decision 9's ninety-day lifetime would leave the
-- new columns empty until November on a library scanned in August -- for data
-- that is already sitting in TMDB's answer. A row written by an older field set
-- counts as stale and is refetched once, in the ordinary scan batches, at the
-- ordinary rate limit.
--
-- Defaults to 0 rather than to the current version on purpose: 0 is exactly
-- what an existing row is, and the backfill is the point.
ALTER TABLE movies ADD COLUMN metadata_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE series ADD COLUMN metadata_version INTEGER NOT NULL DEFAULT 0;
