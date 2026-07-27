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

type streamInfoResponse struct {
	ID        int64  `json:"id"`
	Mode      string `json:"mode"`
	Reason    string `json:"reason,omitempty"`
	Container string `json:"container"`

	// Whether ffmpeg is already on disk. The interface uses it to warn that the
	// first remux will pause to download 80 MB, rather than looking frozen.
	FFmpegReady     bool `json:"ffmpeg_ready"`
	FFmpegSupported bool `json:"ffmpeg_supported"`

	// A remuxed stream is a pipe and carries no duration, so the player cannot
	// draw a seek bar from the file itself. This is where that number comes
	// from: a probe from a previous playback, or TMDB's runtime as a fallback.
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	Progress        *library.Progress `json:"progress,omitempty"`
}

// handleStreamInfo says how a film will be delivered.
//
// Deliberately decided from the container alone: probing means running ffmpeg,
// and running ffmpeg means downloading it. Asking "how will this play" must not
// cost 80 MB for a library that never needs it.
func (s *Server) handleStreamInfo(w http.ResponseWriter, r *http.Request) {
	movie, ok := s.movieForStream(w, r)
	if !ok {
		return
	}

	decision := stream.DecideByContainer(movie.Path)

	// Prefer a duration measured from the file. TMDB's runtime is rounded to
	// the minute and describes the film rather than this copy of it, so it is
	// only a fallback -- but it beats no seek bar at all on a first playback.
	duration := movie.Progress.DurationSeconds
	if duration <= 0 && movie.Metadata.Runtime > 0 {
		duration = float64(movie.Metadata.Runtime) * 60
	}

	progress := movie.Progress
	writeJSON(w, http.StatusOK, streamInfoResponse{
		ID:              movie.ID,
		Mode:            string(decision.Mode),
		Reason:          decision.Reason,
		Container:       strings.TrimPrefix(strings.ToLower(filepath.Ext(movie.Path)), "."),
		FFmpegReady:     s.ffmpeg.Available(),
		FFmpegSupported: ffmpeg.Supported(),
		DurationSeconds: duration,
		Progress:        &progress,
	})
}

// handleStreamDirect serves the file untouched.
//
// http.ServeContent does the whole of HTTP range handling, which is what makes
// the browser's seek bar work: it asks for the byte range around the timestamp
// and gets a 206 back.
func (s *Server) handleStreamDirect(w http.ResponseWriter, r *http.Request) {
	movie, ok := s.movieForStream(w, r)
	if !ok {
		return
	}

	file, err := os.Open(movie.Path)
	if err != nil {
		s.log.Warn("opening a film for direct play failed", "path", movie.Path, "error", err)
		writeJSONError(w, http.StatusNotFound, "the file could not be opened")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "the file could not be read")
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(movie.Path))
	// Private: this is one household's media, and no proxy has any business
	// keeping a copy of it.
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, filepath.Base(movie.Path), info.ModTime(), file)
}

// handleStreamRemux rewraps a file into fragmented MP4 on the fly.
//
// This is the path that needs ffmpeg, and therefore the path that downloads it.
// The response is a pipe, so there is no Content-Length and no byte-range
// seeking; the player seeks by asking for a new stream with ?t=.
func (s *Server) handleStreamRemux(w http.ResponseWriter, r *http.Request) {
	movie, ok := s.movieForStream(w, r)
	if !ok {
		return
	}

	binary, err := s.ffmpeg.Path(r.Context())
	if errors.Is(err, ffmpeg.ErrUnsupportedPlatform) {
		writeJSONError(w, http.StatusNotImplemented,
			"no ffmpeg build is available for this platform, so this file cannot be rewrapped")
		return
	}
	if err != nil {
		s.log.Error("ffmpeg is unavailable", "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg could not be prepared")
		return
	}

	info, err := s.ffmpeg.Probe(r.Context(), movie.Path)
	if err != nil {
		s.log.Warn("probing a film failed", "path", movie.Path, "error", err)
		writeJSONError(w, http.StatusUnsupportedMediaType, "the file could not be read as video")
		return
	}

	decision := stream.Decide(movie.Path, info.VideoCodec, info.AudioCodec)
	if decision.Mode == stream.ModeUnsupported {
		writeJSONError(w, http.StatusUnsupportedMediaType, decision.Reason)
		return
	}

	// The probe is the only place a remuxed file's real duration is ever known,
	// so it is written down here. Every later playback gets a seek bar without
	// paying for another probe.
	if info.Seconds > 0 && movie.Progress.DurationSeconds <= 0 {
		if err := s.lib.SaveDuration(r.Context(), movie.ID, info.Seconds); err != nil {
			s.log.Warn("saving a probed duration failed", "id", movie.ID, "error", err)
		}
	}

	start := 0.0
	if raw := r.URL.Query().Get("t"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			start = parsed
		}
	}

	args := stream.RemuxArgs(movie.Path, decision, start)
	s.log.Info("remuxing",
		"film", movie.Title,
		"video", info.VideoCodec,
		"audio", info.AudioCodec,
		"audio_action", decision.Audio,
		"start", start,
	)

	cmd := exec.CommandContext(r.Context(), binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "the stream could not be started")
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "the stream could not be started")
		return
	}
	// Killed by the context when the viewer navigates away; without this an
	// abandoned tab would leave an ffmpeg running until the film ended.
	defer func() {
		_ = cmd.Wait()
	}()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, stdout); err != nil {
		// The overwhelmingly common cause is the viewer closing the tab, which
		// is not worth an error line.
		s.log.Debug("the remux stream ended early", "error", err)
		return
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		s.log.Warn("ffmpeg reported a problem", "film", movie.Title, "message", msg)
	}
}

// movieForStream resolves the id and checks the file is one we are allowed to
// serve. Writes the error response itself and reports whether to continue.
func (s *Server) movieForStream(w http.ResponseWriter, r *http.Request) (library.Movie, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return library.Movie{}, false
	}

	movie, err := s.lib.Get(r.Context(), id)
	switch {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "no such film")
		return library.Movie{}, false
	case err != nil:
		s.log.Error("reading a film failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the film")
		return library.Movie{}, false
	}

	// The path comes from our own database rather than from the request, so
	// this is belt and braces -- but it is the check that stops a tampered row
	// from turning the player into a file server for the whole disk.
	if !s.withinLibrary(movie.Path) {
		s.log.Error("refused to stream a file outside every configured library directory",
			"path", movie.Path)
		writeJSONError(w, http.StatusForbidden, "this file is outside the library")
		return library.Movie{}, false
	}
	return movie, true
}

func (s *Server) withinLibrary(path string) bool {
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range s.cfg.LibraryPaths {
		base, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(base, target)
		if err != nil {
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// contentTypeFor maps a container to what the browser should be told it is.
func contentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogv", ".ogg":
		return "video/ogg"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
