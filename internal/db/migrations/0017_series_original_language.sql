ALTER TABLE series ADD COLUMN original_language TEXT;
-- Refill cached series through the ordinary bounded metadata queue.
UPDATE series SET metadata_fetched_at = 0 WHERE metadata_status = 'ok';
CREATE INDEX IF NOT EXISTS episode_progress_resume ON episode_progress(profile_id, finished, watched_at DESC, episode_item_id);
