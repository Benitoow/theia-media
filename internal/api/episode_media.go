package api

import (
	"errors"
	"net/http"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

// handleInspectEpisodeFile is the only episode-detail action that may prepare
// ffmpeg. Catalogue and stream-info GETs stay free of downloads and probes.
func (s *Server) handleInspectEpisodeFile(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	if s.ffmpeg == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	info, err := s.ffmpeg.Probe(r.Context(), file.Path)
	switch {
	case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
		writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
		return
	case errors.Is(err, ffmpeg.ErrMediaUnreadable):
		if markErr := s.lib.MarkEpisodeFileMediaError(r.Context(), item.ID, file.ID); markErr != nil {
			s.log.Warn("recording a failed episode inspection failed",
				"episode_id", item.ID, "file_id", file.ID, "error", markErr)
		}
		writeJSONError(w, http.StatusUnsupportedMediaType, "media_unreadable")
		return
	case err != nil:
		s.log.Error("ffmpeg could not inspect an episode file",
			"episode_id", item.ID, "file_id", file.ID, "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	saved, err := s.lib.SaveEpisodeFileMedia(r.Context(), item.ID, file.ID, measuredFileMedia(info))
	if err != nil {
		s.log.Error("saving an episode media inspection failed",
			"episode_id", item.ID, "file_id", file.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "media_inspection_not_saved")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
