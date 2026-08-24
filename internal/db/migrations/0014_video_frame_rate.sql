-- The source frame rate, so "is this browser keeping up" can be a ratio.
--
-- Decision 59 measures presented frames per second of film and compares them to
-- a fixed floor of ten. That constant was chosen for one stated reason: the
-- server did not store what the file actually runs at. It worked for the case it
-- was written from -- a 1080p remux that measured near zero -- and it cannot
-- work for a 4K one, where a decoder managing fourteen frames of a 23.98 fps
-- film is failing badly and sits comfortably above the floor.
--
-- Nothing here is inferred. The value is whatever ffmpeg printed on the stream
-- line, and NULL means it did not say -- in which case the old fixed floor is
-- still the answer.

ALTER TABLE movie_files   ADD COLUMN video_frame_rate REAL;
ALTER TABLE episode_files ADD COLUMN video_frame_rate REAL;
