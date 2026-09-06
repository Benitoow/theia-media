package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/library"
)

type streamInfoResponse struct {
	ID           int64  `json:"id"`
	MovieID      int64  `json:"movie_id,omitempty"`
	EpisodeID    int64  `json:"episode_id,omitempty"`
	FileID       int64  `json:"file_id,omitempty"`
	AudioTrackID int64  `json:"audio_track_id,omitempty"`
	Mode         string `json:"mode"`
	Reason       string `json:"reason,omitempty"`
	ReasonCode   string `json:"reason_code,omitempty"`
	Container    string `json:"container"`
	MediaStatus  string `json:"media_status,omitempty"`
	VideoRisky   bool   `json:"video_risky,omitempty"`

	// The measured video codec, lowercase, e.g. "hevc". Sent so the browser can
	// remember its own verdict per codec rather than for "risky files" as a
	// bucket: whether a machine decodes HEVC says nothing about AV1, and the
	// next codec to join riskyVideo must not inherit an old answer.
	VideoCodec string `json:"video_codec,omitempty"`

	// Whether ffmpeg is already on disk. The interface uses it to warn that the
	// first remux will pause to download 80 MB, rather than looking frozen.
	FFmpegReady     bool `json:"ffmpeg_ready"`
	FFmpegSupported bool `json:"ffmpeg_supported"`

	// A remuxed stream is a pipe and carries no duration, so the player cannot
	// draw a seek bar from the file itself. This is where that number comes
	// from: a probe from a previous playback, or TMDB's runtime as a fallback.
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	Progress        *library.Progress `json:"progress,omitempty"`

	// What can be chosen while watching. The player asks for this instead of
	// inheriting a copy from the page behind it: the tracks of a file that has
	// never been inspected are only measured when playback begins, so a page
	// loaded before that would offer an empty menu until somebody reloaded it.
	AudioTracks    []library.AudioTrack    `json:"audio_tracks,omitempty"`
	SubtitleTracks []library.SubtitleTrack `json:"subtitle_tracks,omitempty"`

	// V2-M6. Qualities lists only what this machine can actually produce for
	// this file, and Transcode says what producing it would cost.
	Height int `json:"height,omitempty"`

	// FrameRate is the file's own cadence, and it travels for one reason: the
	// player compares the frames it manages to present against it. Absent means
	// the file has not been inspected, or ffmpeg did not print one -- the player
	// then falls back to the fixed floor decision 59 shipped with.
	FrameRate float64        `json:"frame_rate,omitempty"`
	Qualities []videoQuality `json:"qualities,omitempty"`
	Transcode *transcodeInfo `json:"transcode,omitempty"`
}

// handleStreamInfo says how a film will be delivered.
//
// Deliberately decided from the container alone: probing means running ffmpeg,
// and running ffmpeg means downloading it. Asking "how will this play" must not
// cost 80 MB for a library that never needs it.
func (s *Server) handleStreamInfo(w http.ResponseWriter, r *http.Request) {
	if !s.selectPrimaryStreamFile(w, r) {
		return
	}
	s.handleMovieFileStreamInfo(w, r)
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

	// Recorded so the updater knows not to restart mid-film. Direct play is a
	// burst of short range requests rather than one long one, which is why the
	// tracker also remembers when the last one finished.
	endPlayback, admitted := s.beginPlayback(w)
	if !admitted {
		return
	}
	defer endPlayback()

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
	if !s.selectPrimaryStreamFile(w, r) {
		return
	}
	s.handleMovieFileStreamRemux(w, r)
}

// Legacy links resolve the primary file, then use the same measured pipeline.
func (s *Server) selectPrimaryStreamFile(w http.ResponseWriter, r *http.Request) bool {
	movie, ok := s.movieForStream(w, r)
	if !ok {
		return false
	}
	for _, file := range movie.Files {
		if file.IsPrimary {
			r.SetPathValue("file_id", strconv.FormatInt(file.ID, 10))
			return true
		}
	}
	writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
	return false
}

// movieForStream resolves the id and checks the file is one we are allowed to
// serve. Writes the error response itself and reports whether to continue.
func (s *Server) movieForStream(w http.ResponseWriter, r *http.Request) (library.Movie, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return library.Movie{}, false
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return library.Movie{}, false
	}

	movie, err := s.lib.Get(r.Context(), profileID, id)
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
	// from turning the player into a file server for the whole disk. Keep the
	// resolved path: opening the unresolved symlink after checking its target
	// would reintroduce a check/open race.
	resolved, inside := s.resolvedLibraryPath(movie.Path)
	if !inside {
		s.log.Error("refused to stream a file outside every configured library directory",
			"path", movie.Path)
		writeJSONError(w, http.StatusForbidden, "this file is outside the library")
		return library.Movie{}, false
	}
	movie.Path = resolved
	return movie, true
}

func (s *Server) resolvedLibraryPath(path string) (string, bool) {
	target, err := resolvedPath(path)
	if err != nil {
		return "", false
	}
	for _, root := range s.libraryRoots() {
		base, err := resolvedPath(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(base, target)
		if err != nil {
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return target, true
		}
	}
	return "", false
}

// resolvedPath closes the symlink/junction hole in a plain filepath.Rel check.
// The file is about to be opened, so failure to resolve an existing target is
// a refusal rather than a reason to fall back to the lexical path.
func resolvedPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	// A file can disappear between the scan and playback. Resolve its existing
	// parent so containment is still checked through junctions/symlinks, then
	// let os.Open produce the useful media_file_unavailable response.
	parent, parentErr := filepath.EvalSymlinks(filepath.Dir(absolute))
	if parentErr != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
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
