package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/subtitles"
)

// The film side of the delivery pair. Identity, tables and route family stay
// here (decision 39); what the route shares with its episode twin lives in
// stream_twin.go, and the delivery policy lives in the playback service.

func (s *Server) handleMovieFileStreamInfo(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamInfo(w, r, s.movieTwin(movie, file))
}

func (s *Server) handleMovieFileStreamDirect(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamDirect(w, r, s.movieTwin(movie, file))
}

func (s *Server) handleMovieFileStreamRemux(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamRemux(w, r, s.movieTwin(movie, file))
}

// movieTwin carries the film side's identity and the library calls that
// differ from the episode side. Everything the closures capture is already
// resolved and contained: the path was checked by movieFileForStream.
func (s *Server) movieTwin(movie library.Movie, file library.MovieFile) streamTwin {
	return streamTwin{
		id:              movie.ID,
		movieID:         movie.ID,
		fileID:          file.ID,
		label:           "film",
		media:           file.Media,
		extension:       file.Extension,
		progress:        movie.Progress,
		path:            file.Path,
		validatesHeight: true,
		audioTrack: func(ctx context.Context, audioID int64) (library.AudioTrack, error) {
			return s.lib.AudioTrack(ctx, movie.ID, file.ID, audioID)
		},
		syncSubtitles: func(ctx context.Context) library.FileMedia {
			// Asked once per playback, which is what keeps a `.srt` dropped in
			// this afternoon offered this evening without a rescan.
			s.syncSidecars(file.Path, file.ID, func(id int64, found []subtitles.Sidecar) error {
				return s.lib.SyncMovieFileSubtitles(ctx, id, found)
			})
			if reloaded, err := s.lib.GetMovieFile(ctx, movie.ID, file.ID); err == nil {
				file.Media.SubtitleTracks = reloaded.Media.SubtitleTracks
			}
			return file.Media
		},
		saveMeasured: func(ctx context.Context, media library.FileMedia) (library.FileMedia, error) {
			saved, err := s.lib.SaveFileMedia(ctx, movie.ID, file.ID, media)
			if err != nil {
				return library.FileMedia{}, err
			}
			return saved.Media, nil
		},
		markUnreadable: func(ctx context.Context) error {
			return s.lib.MarkFileMediaError(ctx, movie.ID, file.ID)
		},
		fallbackDuration: func() float64 {
			if movie.Metadata.Runtime > 0 {
				return float64(movie.Metadata.Runtime) * 60
			}
			return 0
		},
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
