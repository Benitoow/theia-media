package api

import (
	"errors"
	"github.com/Benitoow/theia-media/internal/library"
	"net/http"
	"strconv"
)

func (s *Server) handleWatchlist(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}
	ids, err := s.lib.Watchlist(r.Context(), profile)
	if err != nil {
		writeJSONError(w, 500, "watchlist_unavailable")
		return
	}
	writeJSON(w, 200, struct {
		IDs []int64 `json:"ids"`
	}{ids})
}

func (s *Server) handleSetWatchlist(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, 400, "invalid_movie_id")
		return
	}
	err = s.lib.SetWatchlist(r.Context(), profile, id, r.Method == http.MethodPut)
	if errors.Is(err, library.ErrNoSuchMovie) {
		writeJSONError(w, 404, "movie_not_found")
		return
	}
	if err != nil {
		writeJSONError(w, 500, "watchlist_save_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
