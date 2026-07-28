package api

import (
	"errors"
	"net/http"

	"github.com/Benitoow/theia-media/internal/updater"
)

// handleUpdateStatus reports what the updater knows, without contacting GitHub.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		writeJSONError(w, http.StatusNotFound, "updates are not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.updater.Status())
}

// handleUpdateCheck asks GitHub now rather than waiting for the timer.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		writeJSONError(w, http.StatusNotFound, "updates are not configured")
		return
	}

	status, err := s.updater.Check(r.Context())
	if err != nil {
		// The status carries the explanation; a failed check is not an error
		// worth breaking the settings page over.
		s.log.Debug("a manual update check failed", "error", err)
	}
	writeJSON(w, http.StatusOK, status)
}

// handleUpdateApply installs the new version.
//
// Returns before the restart happens: the response has to reach the browser
// while the server is still able to send it.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		writeJSONError(w, http.StatusNotFound, "updates are not configured")
		return
	}

	err := s.updater.Apply(r.Context())
	switch {
	case errors.Is(err, updater.ErrPlaybackInProgress):
		// Not an error the user did anything wrong to cause, so it is reported
		// as a state rather than a failure.
		writeJSON(w, http.StatusConflict, s.updater.Status())
		return
	case errors.Is(err, updater.ErrBusy):
		writeJSON(w, http.StatusConflict, s.updater.Status())
		return
	case err != nil:
		s.log.Error("applying an update failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, s.updater.Status())
		return
	}
	writeJSON(w, http.StatusOK, s.updater.Status())
}
