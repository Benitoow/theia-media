-- What a file is, as opposed to how it plays.
--
-- The transfer function is the one measurement here that changes behaviour: a
-- PQ or HLG source re-encoded to SDR H.264 without being tone mapped comes out
-- grey and washed out, because the encoder is handed BT.2020 code points and
-- told they are BT.709. Everything else on this migration is a name -- the
-- Dolby Vision record, the audio profile -- kept so a file can say what it
-- holds without a second inspection.
--
-- All of it is NULL or 0 for a library inspected before now, which is correct:
-- nothing here is inferred from a filename, and a file says what it is only
-- once ffmpeg has looked at it again.

ALTER TABLE movie_files   ADD COLUMN video_color_transfer TEXT;
ALTER TABLE movie_files   ADD COLUMN video_dolby_vision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE episode_files ADD COLUMN video_color_transfer TEXT;
ALTER TABLE episode_files ADD COLUMN video_dolby_vision INTEGER NOT NULL DEFAULT 0;

ALTER TABLE movie_file_audio_tracks   ADD COLUMN profile TEXT;
ALTER TABLE episode_file_audio_tracks ADD COLUMN profile TEXT;
