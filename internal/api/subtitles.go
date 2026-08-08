package api

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/subtitles"
)

// handleMovieFileSubtitle serves one subtitle track as WebVTT.
func (s *Server) handleMovieFileSubtitle(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	trackID, ok := pathID(w, r, "track_id", "invalid_subtitle_track_id")
	if !ok {
		return
	}
	track, err := s.lib.MovieSubtitleTrack(r.Context(), movie.ID, file.ID, trackID)
	if !s.subtitleTrackResolved(w, err, movie.ID, file.ID, trackID) {
		return
	}
	s.serveSubtitle(w, r, file.Path, track)
}

// handleEpisodeFileSubtitle is the same contract under the episode routes.
func (s *Server) handleEpisodeFileSubtitle(w http.ResponseWriter, r *http.Request) {
	episode, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	trackID, ok := pathID(w, r, "track_id", "invalid_subtitle_track_id")
	if !ok {
		return
	}
	track, err := s.lib.EpisodeSubtitleTrack(r.Context(), episode.ID, file.ID, trackID)
	if !s.subtitleTrackResolved(w, err, episode.ID, file.ID, trackID) {
		return
	}
	s.serveSubtitle(w, r, file.Path, track)
}

func (s *Server) subtitleTrackResolved(w http.ResponseWriter, err error, parentID, fileID, trackID int64) bool {
	switch {
	case errors.Is(err, library.ErrNoSuchSubtitleTrack):
		writeJSONError(w, http.StatusNotFound, "subtitle_track_not_found")
		return false
	case err != nil:
		s.log.Error("reading a subtitle track failed",
			"parent_id", parentID, "file_id", fileID, "track_id", trackID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "subtitle_track_unavailable")
		return false
	}
	return true
}

// serveSubtitle writes one track out as WebVTT, rebased so that ?t= is zero.
//
// The offset is not cosmetic. A remuxed stream is a pipe restarted at a
// timestamp, so the video element's clock begins again at zero on every seek.
// Subtitles carrying the film's own clock would sit as far from the picture as
// the viewer has travelled into the film. The same number moves both.
func (s *Server) serveSubtitle(w http.ResponseWriter, r *http.Request, mediaPath string, track library.SubtitleTrack) {
	if !track.Renderable() {
		// Decision 3: a bitmap track can only be shown by burning it into the
		// picture, which is the full transcode v1 refuses. Named rather than
		// hidden, so the interface can say which one and why.
		writeJSONError(w, http.StatusUnsupportedMediaType, "subtitle_image_based")
		return
	}

	start := 0.0
	if raw := r.URL.Query().Get("t"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			start = parsed
		}
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")

	if track.IsExternal {
		s.serveExternalSubtitle(w, track, start)
		return
	}
	s.serveEmbeddedSubtitle(w, r, mediaPath, track, start)
}

// serveExternalSubtitle reads a `.srt` from disk and converts it in-process.
//
// No ffmpeg: a browser-friendly MP4 with a subtitle file next to it must gain
// its subtitles without downloading 80 MB to learn what a directory listing
// already said.
func (s *Server) serveExternalSubtitle(w http.ResponseWriter, track library.SubtitleTrack, start float64) {
	path, inside := s.resolvedLibraryPath(track.Path)
	if !inside {
		s.log.Error("refused to read a subtitle outside every configured library directory",
			"track_id", track.ID, "path", track.Path)
		writeJSONError(w, http.StatusForbidden, "file_outside_library")
		return
	}
	handle, err := os.Open(path)
	if err != nil {
		s.log.Warn("opening an external subtitle failed",
			"track_id", track.ID, "path", path, "error", err)
		writeJSONError(w, http.StatusNotFound, "subtitle_unavailable")
		return
	}
	defer handle.Close()

	if err := subtitles.Convert(handle, secondsToDuration(start), w); err != nil {
		// The header is already written by then; there is nothing left to say
		// over HTTP but the log still records which file was malformed.
		s.log.Warn("converting an external subtitle failed",
			"track_id", track.ID, "path", path, "error", err)
	}
}

// serveEmbeddedSubtitle pulls one stream out of the container with ffmpeg.
func (s *Server) serveEmbeddedSubtitle(w http.ResponseWriter, r *http.Request, mediaPath string, track library.SubtitleTrack, start float64) {
	if s.ffmpeg == nil || track.StreamIndex == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}
	binary, err := s.ffmpeg.Path(r.Context())
	switch {
	case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
		writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
		return
	case err != nil:
		s.log.Error("ffmpeg is unavailable for a subtitle", "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	if s.activity != nil {
		defer s.activity.Begin()()
	}

	args := subtitles.ExtractArgs(mediaPath, *track.StreamIndex, start)
	cmd := exec.CommandContext(r.Context(), binary, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		s.log.Warn("extracting a subtitle track failed",
			"track_id", track.ID, "stream_index", *track.StreamIndex,
			"error", err, "message", strings.TrimSpace(stderr.String()))
		writeJSONError(w, http.StatusUnsupportedMediaType, "subtitle_unavailable")
		return
	}
	if len(output) == 0 {
		// A track with no cue left after the seek is still a valid, empty file.
		// An empty body is not: the browser reports a network error for it.
		output = []byte("WEBVTT\n\n")
	}
	if _, err := w.Write(output); err != nil {
		s.log.Debug("a subtitle response ended early", "error", err)
	}
}

// syncSidecars records the `.srt` files currently sitting beside a media file.
//
// It runs when the player asks how a file will be delivered, which is once per
// playback: a subtitle dropped into the folder this afternoon is offered this
// evening without a rescan, and one deleted stops being offered.
func (s *Server) syncSidecars(mediaPath string, fileID int64, save func(int64, []subtitles.Sidecar) error) {
	found, err := subtitles.FindSidecars(mediaPath)
	if err != nil {
		s.log.Debug("looking for subtitle files beside a media file failed",
			"file_id", fileID, "error", err)
		return
	}
	inside := found[:0]
	for _, sidecar := range found {
		if _, ok := s.resolvedLibraryPath(sidecar.Path); ok {
			inside = append(inside, sidecar)
		}
	}
	if err := save(fileID, inside); err != nil {
		s.log.Warn("recording subtitle files beside a media file failed",
			"file_id", fileID, "error", err)
	}
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
