package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Benitoow/theia-media/internal/library"
)

type seriesResponse struct {
	Series []library.Series `json:"series"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (s *Server) handleSeriesList(w http.ResponseWriter, r *http.Request) {
	limit := clamp(intQuery(r, "limit", 100), 1, 500)
	offset := intQuery(r, "offset", 0)
	series, err := s.lib.ListSeries(r.Context(), limit, offset)
	if err != nil {
		s.log.Error("listing series failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "series_unavailable")
		return
	}
	total, err := s.lib.SeriesCount(r.Context())
	if err != nil {
		s.log.Error("counting series failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "series_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, seriesResponse{
		Series: series,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := positivePathID(w, r, "id", "invalid_series_id")
	if !ok {
		return
	}
	series, err := s.lib.GetSeries(r.Context(), id)
	switch {
	case errors.Is(err, library.ErrNoSuchSeries):
		writeJSONError(w, http.StatusNotFound, "series_not_found")
	case err != nil:
		s.log.Error("reading series failed", "series_id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "series_unavailable")
	default:
		writeJSON(w, http.StatusOK, series)
	}
}

func (s *Server) handleSeason(w http.ResponseWriter, r *http.Request) {
	seriesID, ok := positivePathID(w, r, "id", "invalid_series_id")
	if !ok {
		return
	}
	number, err := strconv.Atoi(r.PathValue("season"))
	if err != nil || number < 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_season_number")
		return
	}
	season, err := s.lib.GetSeason(r.Context(), seriesID, number)
	switch {
	case errors.Is(err, library.ErrNoSuchSeason):
		writeJSONError(w, http.StatusNotFound, "season_not_found")
	case err != nil:
		s.log.Error("reading season failed", "series_id", seriesID, "season", number, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "season_unavailable")
	default:
		writeJSON(w, http.StatusOK, season)
	}
}

func (s *Server) handleEpisode(w http.ResponseWriter, r *http.Request) {
	id, ok := positivePathID(w, r, "id", "invalid_episode_id")
	if !ok {
		return
	}
	episode, err := s.lib.GetEpisodeItem(r.Context(), id)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "episode_not_found")
	case err != nil:
		s.log.Error("reading episode failed", "episode_id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "episode_unavailable")
	default:
		writeJSON(w, http.StatusOK, episode)
	}
}

func (s *Server) handleSeriesHome(w http.ResponseWriter, r *http.Request) {
	limit := clamp(intQuery(r, "limit", 12), 1, 60)
	home, err := s.lib.SeriesHome(r.Context(), limit)
	if err != nil {
		s.log.Error("building series home failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "series_home_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, home)
}

func (s *Server) handleSaveEpisodeProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := positivePathID(w, r, "id", "invalid_episode_id")
	if !ok {
		return
	}
	var body progressRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_progress_payload")
		return
	}
	progress, err := s.lib.SaveEpisodeProgress(r.Context(), id,
		body.PositionSeconds, body.DurationSeconds)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "episode_not_found")
	case err != nil:
		s.log.Error("saving episode progress failed", "episode_id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "progress_not_saved")
	default:
		writeJSON(w, http.StatusOK, progress)
	}
}

func (s *Server) handleResetEpisodeProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := positivePathID(w, r, "id", "invalid_episode_id")
	if !ok {
		return
	}
	switch err := s.lib.ResetEpisodeProgress(r.Context(), id); {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "episode_not_found")
	case err != nil:
		s.log.Error("resetting episode progress failed", "episode_id", id, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "progress_not_reset")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func positivePathID(w http.ResponseWriter, r *http.Request, name, code string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, code)
		return 0, false
	}
	return id, true
}
