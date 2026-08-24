package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
	"github.com/Benitoow/theia-media/internal/subtitles"
)

func (s *Server) handleMovieFileStreamInfo(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}

	audioID, audioRequested, ok := requestedAudioID(w, r)
	if !ok {
		return
	}

	var selected *library.AudioTrack
	if audioRequested {
		if file.Media.Status != library.MediaOK {
			writeJSONError(w, http.StatusConflict, "media_not_inspected")
			return
		}
		track, err := s.lib.AudioTrack(r.Context(), movie.ID, file.ID, audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			s.log.Error("reading an audio track failed",
				"film_id", movie.ID, "file_id", file.ID, "track_id", audioID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "audio_track_unavailable")
			return
		}
		selected = &track
	}

	decision := stream.DecideByContainer(file.Path)
	reasonCode := "container_remux"
	if decision.Mode == stream.ModeDirect {
		reasonCode = "container_direct"
	}
	if file.Media.Status == library.MediaOK && file.Media.Video != nil {
		audioCodec := ""
		if selected != nil {
			audioCodec = selected.Codec
		} else if track, ok := stream.PreferredAudio(file.Media.AudioTracks, func(t library.AudioTrack) string { return t.Codec }); ok {
			audioCodec = track.Codec
		}
		decision = stream.Decide(file.Path, file.Media.Video.Codec, audioCodec)
		reasonCode = decisionReasonCode(decision)
	}
	if audioRequested && decision.Mode == stream.ModeDirect {
		// Direct play hands the whole file to the browser and cannot guarantee a
		// chosen track. A manual selection therefore takes the remux route even
		// when the MP4 itself would otherwise play untouched.
		decision.Mode = stream.ModeRemux
		decision.Audio = stream.AudioCopy
		decision.Reason = "a selected audio track must be mapped explicitly"
		reasonCode = "audio_track_selected"
	}

	duration := file.Media.DurationSeconds
	if duration <= 0 {
		duration = movie.Progress.DurationSeconds
	}
	if duration <= 0 && movie.Metadata.Runtime > 0 {
		duration = float64(movie.Metadata.Runtime) * 60
	}
	container := file.Media.Container
	if container == "" {
		container = file.Extension
	}
	// Asked once per playback, which is what keeps a `.srt` dropped in this
	// afternoon offered this evening without a rescan.
	s.syncSidecars(file.Path, file.ID, func(id int64, found []subtitles.Sidecar) error {
		return s.lib.SyncMovieFileSubtitles(r.Context(), id, found)
	})
	if reloaded, err := s.lib.GetMovieFile(r.Context(), movie.ID, file.ID); err == nil {
		file.Media.SubtitleTracks = reloaded.Media.SubtitleTracks
	}

	// V2-M6. A picture no browser decodes stops being a refusal when this
	// machine has an encoder that runs.
	capabilities := s.videoCapabilities(r.Context())
	ladder := qualityLadder(file.Media, decision, capabilities.Available)
	if decision.Mode == stream.ModeUnsupported && capabilities.Available {
		decision.Mode = stream.ModeTranscode
		reasonCode = "video_transcode"
	}
	height, frameRate := 0, 0.0
	if file.Media.Video != nil {
		height = file.Media.Video.Height
		frameRate = file.Media.Video.FrameRate
	}

	progress := movie.Progress
	writeJSON(w, http.StatusOK, streamInfoResponse{
		ID:              movie.ID,
		MovieID:         movie.ID,
		FileID:          file.ID,
		AudioTrackID:    audioID,
		Mode:            string(decision.Mode),
		ReasonCode:      reasonCode,
		Container:       container,
		MediaStatus:     file.Media.Status,
		VideoRisky:      decision.VideoRisky,
		VideoCodec:      videoCodecOf(file.Media),
		FFmpegReady:     s.ffmpeg != nil && s.ffmpeg.Available(),
		FFmpegSupported: ffmpeg.Supported(),
		DurationSeconds: duration,
		Progress:        &progress,
		AudioTracks:     file.Media.AudioTracks,
		SubtitleTracks:  file.Media.SubtitleTracks,
		Height:          height,
		FrameRate:       frameRate,
		Qualities:       ladder,
		Transcode:       &capabilities,
	})
}

func (s *Server) handleMovieFileStreamDirect(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Has("audio") {
		writeJSONError(w, http.StatusBadRequest, "audio_selection_requires_remux")
		return
	}

	if s.activity != nil {
		defer s.activity.Begin()()
	}
	opened, err := os.Open(file.Path)
	if err != nil {
		s.log.Warn("opening a film file for direct play failed",
			"film_id", movie.ID, "file_id", file.ID, "path", file.Path, "error", err)
		writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
		return
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "media_file_unreadable")
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(file.Path))
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, filepath.Base(file.Path), info.ModTime(), opened)
}

func (s *Server) handleMovieFileStreamRemux(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	audioID, audioRequested, ok := requestedAudioID(w, r)
	if !ok {
		return
	}
	wantedHeight, heightRequested, ok := requestedHeight(w, r)
	if !ok {
		return
	}
	if s.ffmpeg == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	if s.activity != nil {
		defer s.activity.Begin()()
	}

	// SubtitlesScanned is the third condition, and it only ever fires once per
	// file: a library inspected before subtitles existed has every measurement
	// except that one, and re-probing here costs a single ffmpeg run on a path
	// that was about to start one anyway.
	if file.Media.Status != library.MediaOK || file.Media.Video == nil || !file.Media.SubtitlesScanned {
		info, err := s.ffmpeg.Probe(r.Context(), file.Path)
		switch {
		case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
			writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
			return
		case errors.Is(err, ffmpeg.ErrMediaUnreadable):
			_ = s.lib.MarkFileMediaError(r.Context(), movie.ID, file.ID)
			s.log.Warn("probing a selected film file failed",
				"film_id", movie.ID, "file_id", file.ID, "path", file.Path, "error", err)
			writeJSONError(w, http.StatusUnsupportedMediaType, "media_unreadable")
			return
		case err != nil:
			s.log.Error("ffmpeg could not inspect a selected film file",
				"film_id", movie.ID, "file_id", file.ID, "error", err)
			writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
			return
		}
		file, err = s.lib.SaveFileMedia(r.Context(), movie.ID, file.ID, measuredFileMedia(info))
		if err != nil {
			s.log.Error("saving a selected file inspection failed",
				"film_id", movie.ID, "file_id", file.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "media_inspection_not_saved")
			return
		}
	}

	var selected *library.AudioTrack
	if audioRequested {
		track, err := s.lib.AudioTrack(r.Context(), movie.ID, file.ID, audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			s.log.Error("reading a selected audio track failed",
				"film_id", movie.ID, "file_id", file.ID, "track_id", audioID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "audio_track_unavailable")
			return
		}
		selected = &track
	}

	audioCodec := ""
	if selected != nil {
		audioCodec = selected.Codec
	} else if track, ok := stream.PreferredAudio(file.Media.AudioTracks, func(t library.AudioTrack) string { return t.Codec }); ok {
		// Not a silent quality decision: the track the browser can decode is
		// mapped explicitly so a DTS-first BluRay does not burn a core
		// transcoding past an AAC track sitting one index away.
		audioCodec = track.Codec
		defaulted := track
		selected = &defaulted
	}
	decision := stream.Decide(file.Path, file.Media.Video.Codec, audioCodec)

	// V2-M6. The picture is re-encoded when the browser cannot decode it at all,
	// or when somebody has asked for a smaller one. Both need an encoder that
	// runs on this machine -- probed, never assumed.
	needsTranscode := decision.Mode == stream.ModeUnsupported || heightRequested || forcedTranscode(r)
	var encoder ffmpeg.Encoder
	if needsTranscode {
		best, available := s.ffmpeg.Capabilities(r.Context()).Best()
		if !available {
			writeJSONError(w, http.StatusUnsupportedMediaType, "video_transcode_required")
			return
		}
		encoder = best
		s.transcodes.setKind(encoder.Kind)

		// The furnace is lit once at a time in software. Refusing is the honest
		// answer: two concurrent software transcodes do not run at half speed,
		// they both stall, and nobody watching either one can tell why.
		release, got := s.transcodes.acquire()
		if !got {
			writeJSONError(w, http.StatusServiceUnavailable, "transcode_busy")
			return
		}
		defer release()
		decision.Mode = stream.ModeTranscode
	}

	if audioRequested && decision.Mode == stream.ModeDirect {
		decision.Mode = stream.ModeRemux
		decision.Audio = stream.AudioCopy
	}

	binary, err := s.ffmpeg.Path(r.Context())
	switch {
	case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
		writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
		return
	case err != nil:
		s.log.Error("ffmpeg is unavailable", "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	start := 0.0
	if raw := r.URL.Query().Get("t"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			start = parsed
		}
	}
	selectedIndex := -1
	var audioStreamIndex *int
	if selected != nil {
		selectedIndex = selected.StreamIndex
		audioStreamIndex = &selected.StreamIndex
	}

	var args []string
	switch {
	case decision.Mode == stream.ModeTranscode:
		// Probed on the first transcode and remembered, like the encoder. An
		// empty answer means decode on the CPU, which is correct everywhere.
		args = stream.TranscodeArgs(file.Path, decision, start,
			encoder.Name, s.ffmpeg.HardwareDecoder(r.Context()), wantedHeight, audioStreamIndex)
	case audioStreamIndex != nil:
		args = stream.RemuxArgsForAudio(file.Path, decision, start, *audioStreamIndex)
	default:
		args = stream.RemuxArgs(file.Path, decision, start)
	}

	s.log.Info("streaming selected film file",
		"film_id", movie.ID,
		"file_id", file.ID,
		"mode", decision.Mode,
		"video", file.Media.Video.Codec,
		"video_encoder", encoder.Name,
		"height", wantedHeight,
		"audio", audioCodec,
		"audio_track_id", audioID,
		"audio_stream_index", selectedIndex,
		"audio_action", decision.Audio,
		"start", start,
	)

	cmd := exec.CommandContext(r.Context(), binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "stream_start_failed")
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "stream_start_failed")
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, stdout)
	waitErr := cmd.Wait()
	if copyErr != nil {
		s.log.Debug("the selected remux stream ended early", "error", copyErr)
		return
	}
	if waitErr != nil {
		s.log.Warn("ffmpeg ended a selected remux with an error",
			"film_id", movie.ID, "file_id", file.ID,
			"error", waitErr, "message", strings.TrimSpace(stderr.String()))
		return
	}
	if message := strings.TrimSpace(stderr.String()); message != "" {
		s.log.Warn("ffmpeg reported a problem for a selected film file",
			"film_id", movie.ID, "file_id", file.ID, "message", message)
	}
}

func (s *Server) movieFileForStream(w http.ResponseWriter, r *http.Request) (library.Movie, library.MovieFile, bool) {
	movieID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || movieID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_movie_id")
		return library.Movie{}, library.MovieFile{}, false
	}
	fileID, err := strconv.ParseInt(r.PathValue("file_id"), 10, 64)
	if err != nil || fileID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_file_id")
		return library.Movie{}, library.MovieFile{}, false
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return library.Movie{}, library.MovieFile{}, false
	}

	movie, err := s.lib.Get(r.Context(), profileID, movieID)
	switch {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "movie_not_found")
		return library.Movie{}, library.MovieFile{}, false
	case err != nil:
		s.log.Error("reading a film for a file request failed", "id", movieID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "movie_unavailable")
		return library.Movie{}, library.MovieFile{}, false
	}
	file, err := s.lib.GetMovieFile(r.Context(), movieID, fileID)
	switch {
	case errors.Is(err, library.ErrNoSuchMovieFile):
		writeJSONError(w, http.StatusNotFound, "file_not_found")
		return library.Movie{}, library.MovieFile{}, false
	case err != nil:
		s.log.Error("reading a film file failed",
			"film_id", movieID, "file_id", fileID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "file_unavailable")
		return library.Movie{}, library.MovieFile{}, false
	}
	resolved, inside := s.resolvedLibraryPath(file.Path)
	if !inside {
		s.log.Error("refused to use a file outside every configured library directory",
			"film_id", movieID, "file_id", fileID, "path", file.Path)
		writeJSONError(w, http.StatusForbidden, "file_outside_library")
		return library.Movie{}, library.MovieFile{}, false
	}
	file.Path = resolved
	if _, err := os.Stat(file.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
		} else {
			s.log.Warn("reading a selected film file failed",
				"film_id", movieID, "file_id", fileID, "path", file.Path, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "media_file_unreadable")
		}
		return library.Movie{}, library.MovieFile{}, false
	}
	return movie, file, true
}

func requestedAudioID(w http.ResponseWriter, r *http.Request) (int64, bool, bool) {
	raw, present := r.URL.Query()["audio"]
	if !present {
		return 0, false, true
	}
	if len(raw) != 1 || raw[0] == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_audio_track_id")
		return 0, false, false
	}
	id, err := strconv.ParseInt(raw[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_audio_track_id")
		return 0, false, false
	}
	return id, true, true
}

// videoCodecOf is the measured codec, or empty when the file has not been
// inspected. Lowercase, because it is a key the browser stores its own verdict
// under and "HEVC" and "hevc" must not become two answers.
func videoCodecOf(media library.FileMedia) string {
	if media.Status != library.MediaOK || media.Video == nil {
		return ""
	}
	return strings.ToLower(media.Video.Codec)
}

func decisionReasonCode(decision stream.Decision) string {
	switch {
	case decision.Mode == stream.ModeUnsupported:
		return "video_transcode_required"
	case decision.Mode == stream.ModeDirect:
		return "direct_play"
	case decision.Audio == stream.AudioTranscode:
		return "audio_transcode"
	default:
		return "container_remux"
	}
}
