-- A correction survives the next scan.
--
-- Metadata identity was decided entirely by searching TMDB for the parsed title
-- and year, on every refresh. That is right the overwhelming majority of the
-- time and wrong in a way nobody could fix: the README's answer to a mismatched
-- film was to rename the file, which is not something anybody can do from the
-- television they are holding the remote for.
--
-- A pinned row says "the identity is settled, stop searching". It does not say
-- "stop refreshing": decision 9 keeps metadata cached and never frozen, so a
-- pinned film still re-reads its record when the record ages out — by id
-- instead of by title, which is the whole difference.
--
-- Defaults to 0, so every existing row keeps behaving exactly as it does today.
ALTER TABLE movies ADD COLUMN tmdb_locked INTEGER NOT NULL DEFAULT 0
    CHECK (tmdb_locked IN (0, 1));

ALTER TABLE series ADD COLUMN tmdb_locked INTEGER NOT NULL DEFAULT 0
    CHECK (tmdb_locked IN (0, 1));
