package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/library"
)

type moviesResponse struct {
	Movies []library.Movie `json:"movies"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// handleMovies returns a page of the library, ordered by title.
func (s *Server) handleMovies(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 100)
	offset := intQuery(r, "offset", 0)

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	movies, err := s.lib.List(r.Context(), profileID, limit, offset)
	if err != nil {
		s.log.Error("listing the library failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}
	total, err := s.lib.Count(r.Context())
	if err != nil {
		s.log.Error("counting the library failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}

	writeJSON(w, http.StatusOK, moviesResponse{
		Movies: movies,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

type statsResponse struct {
	Movies       int                 `json:"movies"`
	Series       int                 `json:"series"`
	Episodes     int                 `json:"episodes"`
	Scanning     bool                `json:"scanning"`
	LibraryPaths int                 `json:"library_paths"`
	LastScan     *library.ScanReport `json:"last_scan"`
}

// handleLibraryStats is what the home screen polls: how much is in the library,
// whether a scan is running, and how the last one went.
func (s *Server) handleLibraryStats(w http.ResponseWriter, r *http.Request) {
	count, err := s.lib.Count(r.Context())
	if err != nil {
		s.log.Error("counting the library failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}
	series, err := s.lib.SeriesCount(r.Context())
	if err != nil {
		s.log.Error("counting series failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}
	episodes, err := s.lib.EpisodeCount(r.Context())
	if err != nil {
		s.log.Error("counting episodes failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}

	writeJSON(w, http.StatusOK, statsResponse{
		Movies:       count,
		Series:       series,
		Episodes:     episodes,
		Scanning:     s.lib.Scanning(),
		LibraryPaths: len(s.cfg.LibraryPaths),
		LastScan:     s.lib.LastScan(),
	})
}

// handleScan runs a scan and returns its report.
//
// It blocks until the scan finishes, which is honest for a first version: a
// library of a few thousand files takes seconds, and a progress stream is worth
// building once there is a real interface to show it in.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	report, err := s.lib.Scan(r.Context(), s.libraryRoots())
	switch {
	case errors.Is(err, library.ErrScanInProgress):
		writeJSONError(w, http.StatusConflict, "a scan is already running")
		return
	case err != nil:
		s.log.Error("scan failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the scan could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleHome returns the hero and every row in one request.
//
// Rows are short by default. The home screen suggests; /films inventories, and
// the "see all" link on each row is the way from one to the other.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	perRow := clamp(intQuery(r, "per_row", 12), 1, 60)

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	home, err := s.lib.HomeScreen(r.Context(), profileID, perRow)
	if err != nil {
		s.log.Error("building the home screen failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the library")
		return
	}
	writeJSON(w, http.StatusOK, home)
}

// handleMovie returns one film.
func (s *Server) handleMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid film id")
		return
	}

	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	movie, err := s.lib.Get(r.Context(), profileID, id)
	switch {
	case errors.Is(err, library.ErrNoSuchMovie):
		writeJSONError(w, http.StatusNotFound, "no such film")
		return
	case err != nil:
		s.log.Error("reading a film failed", "id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "could not read the film")
		return
	}
	writeJSON(w, http.StatusOK, movie)
}

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// intQuery reads a positive integer query parameter, falling back to a default
// when it is missing or unparseable. A malformed limit is not worth a 400.
func intQuery(r *http.Request, name string, fallback int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

// handleSearch answers one query across films and series.
//
// This exists because the catalogue was searchable in two places and neither
// knew about the other: you had to decide whether the thing you half-remembered
// was a film or a series before you were allowed to look for it.
//
// Searching on the server rather than filtering a downloaded catalogue is also
// what makes this usable from outside the house. A phone on the WireGuard
// listener asks a question and gets twenty rows back, instead of pulling the
// whole library down to grep it locally.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	// A ceiling on the query itself, not on the work: nothing sensible is this
	// long, and the folding walks every rune it is given.
	if len(query) > 200 {
		query = query[:200]
	}

	results, err := s.lib.Search(r.Context(), profileID, query)
	if err != nil {
		s.log.Error("the search failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the library could not be searched")
		return
	}
	// Never null: the interface iterates these without checking.
	if results.Movies == nil {
		results.Movies = []library.Movie{}
	}
	if results.Series == nil {
		results.Series = []library.Series{}
	}
	writeJSON(w, http.StatusOK, results)
}
