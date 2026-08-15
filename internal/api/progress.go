package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Benitoow/theia-media/internal/library"
)

type progressRequest struct {
	PositionSeconds float64 `json:"position_seconds"`

	// The player reports the duration when it knows one, which for a direct
	// stream it does and for a remuxed one it does not. Zero means "keep
	// whatever the server already has".
	DurationSeconds float64 `json:"duration_seconds"`
}

// handleSaveProgress records a playback position.
//
// Called every few seconds while a film plays, so it stays deliberately cheap:
// one UPDATE, no read-modify-write, and a malformed body is a 400 rather than
// anything that could interrupt playback.
func (s *Server) handleSaveProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return
	}

	var body progressRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid progress payload")
		return
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	progress, err := s.lib.SaveProgress(r.Context(), profileID, id, body.PositionSeconds, body.DurationSeconds)
	switch {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "no such film")
		return
	case err != nil:
		s.log.Error("saving progress failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the position could not be saved")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

// handleResetProgress forgets a viewing position, which is what "start from the
// beginning" does.
func (s *Server) handleResetProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	switch err := s.lib.ResetProgress(r.Context(), profileID, id); {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "no such film")
		return
	case err != nil:
		s.log.Error("resetting progress failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the position could not be reset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSetWatched marks a film watched without it having been played.
//
// Two things need this and neither is playback. A film abandoned twenty minutes
// in sits in the continue-watching row forever, because nothing else ever
// finishes it; and a film seen somewhere else is not on this server's record at
// all. Both are the viewer telling the library something it cannot observe.
func (s *Server) handleSetWatched(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	progress, err := s.lib.SetWatched(r.Context(), profileID, id)
	switch {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "no such film")
		return
	case err != nil:
		s.log.Error("marking a film watched failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the film could not be marked watched")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

// handleSetEpisodeWatched marks one episode watched. Same reasoning, and the
// case is more common: a series watched elsewhere up to episode six.
func (s *Server) handleSetEpisodeWatched(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid episode id")
		return
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	progress, err := s.lib.SetEpisodeWatched(r.Context(), profileID, id)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "no such episode")
		return
	case err != nil:
		s.log.Error("marking an episode watched failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the episode could not be marked watched")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}
