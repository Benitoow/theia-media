package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/tmdb"
)

// Correcting a match is administration, and stays on the LAN. It changes what
// the library says a file is, for everybody, and it spends TMDB requests. See
// decision 44, and remoteRouteAllowed, which refuses these paths.

type candidatesResponse struct {
	Candidates []tmdb.Candidate `json:"candidates"`
}

type matchRequest struct {
	TMDBID int `json:"tmdb_id"`
}

// handleMovieCandidates offers the films a title could have meant.
func (s *Server) handleMovieCandidates(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid film id")
	if !ok {
		return
	}

	candidates, err := s.lib.MovieCandidates(r.Context(), id, searchQuery(r))
	if !s.writeCandidates(w, candidates, err, "film", id) {
		return
	}
}

// handleSetMovieMatch pins a film to the record somebody picked.
func (s *Server) handleSetMovieMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid film id")
	if !ok {
		return
	}
	tmdbID, ok := decodeMatchRequest(w, r)
	if !ok {
		return
	}
	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	movie, err := s.lib.SetMovieMatch(r.Context(), profileID, id, tmdbID)
	if err != nil {
		s.writeMatchError(w, err, "film", id)
		return
	}
	s.log.Info("a film was matched by hand", "id", id, "tmdb_id", tmdbID)
	writeJSON(w, http.StatusOK, movie)
}

// handleClearMovieMatch hands a film back to the automatic matcher.
func (s *Server) handleClearMovieMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid film id")
	if !ok {
		return
	}
	if err := s.lib.ClearMovieMatch(r.Context(), id); err != nil {
		s.writeMatchError(w, err, "film", id)
		return
	}
	// The identity is now unsettled, and the next pass decides it. Asking for
	// one straight away means the poster is right again in a minute rather than
	// whenever the folder next changes.
	if s.watcher != nil {
		s.watcher.Wake()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSeriesCandidates(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid series id")
	if !ok {
		return
	}

	candidates, err := s.lib.SeriesCandidates(r.Context(), id, searchQuery(r))
	if !s.writeCandidates(w, candidates, err, "series", id) {
		return
	}
}

func (s *Server) handleSetSeriesMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid series id")
	if !ok {
		return
	}
	tmdbID, ok := decodeMatchRequest(w, r)
	if !ok {
		return
	}
	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	series, err := s.lib.SetSeriesMatch(r.Context(), profileID, id, tmdbID)
	if err != nil {
		s.writeMatchError(w, err, "series", id)
		return
	}
	s.log.Info("a series was matched by hand", "id", id, "tmdb_id", tmdbID)
	writeJSON(w, http.StatusOK, series)
}

func (s *Server) handleClearSeriesMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "invalid series id")
	if !ok {
		return
	}
	if err := s.lib.ClearSeriesMatch(r.Context(), id); err != nil {
		s.writeMatchError(w, err, "series", id)
		return
	}
	if s.watcher != nil {
		s.watcher.Wake()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeCandidates(w http.ResponseWriter, candidates []tmdb.Candidate, err error, kind string, id int64) bool {
	if err != nil {
		s.writeMatchError(w, err, kind, id)
		return false
	}
	if candidates == nil {
		candidates = []tmdb.Candidate{}
	}
	writeJSON(w, http.StatusOK, candidatesResponse{Candidates: candidates})
	return true
}

// writeMatchError maps what can go wrong onto codes the interface owns the
// wording for. Every one of these is a different sentence to a user: no key
// configured, a key the service rejected, a service that is not answering.
func (s *Server) writeMatchError(w http.ResponseWriter, err error, kind string, id int64) {
	switch {
	case errors.Is(err, library.ErrNoSuchMovie), errors.Is(err, library.ErrNoSuchSeries):
		writeJSONError(w, http.StatusNotFound, "no_such_item")
	case errors.Is(err, library.ErrNoMetadataSource):
		writeJSONError(w, http.StatusConflict, "metadata_source_missing")
	case errors.Is(err, tmdb.ErrUnauthorized):
		writeJSONError(w, http.StatusBadGateway, "metadata_key_rejected")
	case errors.Is(err, tmdb.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "metadata_not_found")
	default:
		s.log.Error("a match correction failed", "kind", kind, "id", id, "error", err)
		writeJSONError(w, http.StatusBadGateway, "metadata_unavailable")
	}
}

func parsePathID(w http.ResponseWriter, r *http.Request, message string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, message)
		return 0, false
	}
	return id, true
}

func decodeMatchRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	var body matchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid match payload")
		return 0, false
	}
	if body.TMDBID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid match payload")
		return 0, false
	}
	return body.TMDBID, true
}

func searchQuery(r *http.Request) string {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		query = query[:200]
	}
	return query
}
