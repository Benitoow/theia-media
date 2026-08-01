-- Return installations that ran v1.3.0-v1.5.0 to the single-viewer schema.
--
-- The legacy columns on movies were deliberately kept in sync for the default
-- viewer, so the one playback position Theia keeps remains intact. Extra
-- viewer histories and uploaded avatars are deleted with the retired feature.

DROP TRIGGER IF EXISTS sync_legacy_progress_to_default;
DROP TABLE IF EXISTS playback_progress;
DROP TABLE IF EXISTS profiles;

-- Keep the historical migration marker. A manually launched older binary must
-- not recreate the retired schema after this cleanup has run.
