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
)

func (s *Server) handleEpisodeFileStreamInfo(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
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
		track, err := s.lib.EpisodeAudioTrack(r.Context(), item.ID, file.ID, audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			s.log.Error("reading an episode audio track failed",
				"episode_id", item.ID, "file_id", file.ID, "track_id", audioID, "error", err)
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
		} else if len(file.Media.AudioTracks) > 0 {
			audioCodec = file.Media.AudioTracks[0].Codec
		}
		decision = stream.Decide(file.Path, file.Media.Video.Codec, audioCodec)
		reasonCode = decisionReasonCode(decision)
	}
	if audioRequested && decision.Mode == stream.ModeDirect {
		decision.Mode = stream.ModeRemux
		decision.Audio = stream.AudioCopy
		decision.Reason = "a selected audio track must be mapped explicitly"
		reasonCode = "audio_track_selected"
	}

	duration := file.Media.DurationSeconds
	if duration <= 0 {
		duration = item.Progress.DurationSeconds
	}
	if duration <= 0 {
		duration = episodeRuntimeSeconds(item)
	}
	container := file.Media.Container
	if container == "" {
		container = file.Extension
	}
	progress := item.Progress
	writeJSON(w, http.StatusOK, streamInfoResponse{
		ID:              item.ID,
		EpisodeID:       item.ID,
		FileID:          file.ID,
		AudioTrackID:    audioID,
		Mode:            string(decision.Mode),
		Reason:          decision.Reason,
		ReasonCode:      reasonCode,
		Container:       container,
		MediaStatus:     file.Media.Status,
		VideoRisky:      decision.VideoRisky,
		FFmpegReady:     s.ffmpeg != nil && s.ffmpeg.Available(),
		FFmpegSupported: ffmpeg.Supported(),
		DurationSeconds: duration,
		Progress:        &progress,
	})
}

func episodeRuntimeSeconds(item library.EpisodeItem) float64 {
	minutes := 0
	for _, episode := range item.Episodes {
		minutes += episode.Metadata.RuntimeMinutes
	}
	return float64(minutes) * 60
}

func (s *Server) handleEpisodeFileStreamDirect(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
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
		s.log.Warn("opening an episode file for direct play failed",
			"episode_id", item.ID, "file_id", file.ID, "path", file.Path, "error", err)
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

func (s *Server) handleEpisodeFileStreamRemux(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	audioID, audioRequested, ok := requestedAudioID(w, r)
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

	if file.Media.Status != library.MediaOK || file.Media.Video == nil {
		info, err := s.ffmpeg.Probe(r.Context(), file.Path)
		switch {
		case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
			writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
			return
		case errors.Is(err, ffmpeg.ErrMediaUnreadable):
			_ = s.lib.MarkEpisodeFileMediaError(r.Context(), item.ID, file.ID)
			writeJSONError(w, http.StatusUnsupportedMediaType, "media_unreadable")
			return
		case err != nil:
			s.log.Error("ffmpeg could not inspect an episode file",
				"episode_id", item.ID, "file_id", file.ID, "error", err)
			writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
			return
		}
		file, err = s.lib.SaveEpisodeFileMedia(r.Context(), item.ID, file.ID, measuredFileMedia(info))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "media_inspection_not_saved")
			return
		}
	}

	var selected *library.AudioTrack
	if audioRequested {
		track, err := s.lib.EpisodeAudioTrack(r.Context(), item.ID, file.ID, audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "audio_track_unavailable")
			return
		}
		selected = &track
	}

	audioCodec := ""
	if selected != nil {
		audioCodec = selected.Codec
	} else if len(file.Media.AudioTracks) > 0 {
		audioCodec = file.Media.AudioTracks[0].Codec
	}
	decision := stream.Decide(file.Path, file.Media.Video.Codec, audioCodec)
	if decision.Mode == stream.ModeUnsupported {
		writeJSONError(w, http.StatusUnsupportedMediaType, "video_transcode_required")
		return
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
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	start := 0.0
	if raw := r.URL.Query().Get("t"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			start = parsed
		}
	}
	args := stream.RemuxArgs(file.Path, decision, start)
	selectedIndex := -1
	if selected != nil {
		selectedIndex = selected.StreamIndex
		args = stream.RemuxArgsForAudio(file.Path, decision, start, selected.StreamIndex)
	}
	s.log.Info("remuxing episode file",
		"episode_id", item.ID, "file_id", file.ID,
		"video", file.Media.Video.Codec, "audio", audioCodec,
		"audio_track_id", audioID, "audio_stream_index", selectedIndex,
		"audio_action", decision.Audio, "start", start)

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
		s.log.Debug("the episode remux stream ended early", "error", copyErr)
		return
	}
	if waitErr != nil {
		s.log.Warn("ffmpeg ended an episode remux with an error",
			"episode_id", item.ID, "file_id", file.ID,
			"error", waitErr, "message", strings.TrimSpace(stderr.String()))
		return
	}
	if message := strings.TrimSpace(stderr.String()); message != "" {
		s.log.Warn("ffmpeg reported a problem for an episode file",
			"episode_id", item.ID, "file_id", file.ID, "message", message)
	}
}

func (s *Server) episodeFileForStream(w http.ResponseWriter, r *http.Request) (library.EpisodeItem, library.EpisodeFile, bool) {
	itemID, ok := positivePathID(w, r, "id", "invalid_episode_id")
	if !ok {
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	fileID, ok := positivePathID(w, r, "file_id", "invalid_file_id")
	if !ok {
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	item, err := s.lib.GetEpisodeItem(r.Context(), itemID)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "episode_not_found")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	case err != nil:
		s.log.Error("reading an episode for a file request failed", "episode_id", itemID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "episode_unavailable")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	file, err := s.lib.GetEpisodeFile(r.Context(), itemID, fileID)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeFile):
		writeJSONError(w, http.StatusNotFound, "file_not_found")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	case err != nil:
		s.log.Error("reading an episode file failed",
			"episode_id", itemID, "file_id", fileID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "file_unavailable")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	resolved, inside := s.resolvedLibraryPath(file.Path)
	if !inside {
		writeJSONError(w, http.StatusForbidden, "file_outside_library")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	file.Path = resolved
	if _, err := os.Stat(file.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
		} else {
			writeJSONError(w, http.StatusInternalServerError, "media_file_unreadable")
		}
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	return item, file, true
}
