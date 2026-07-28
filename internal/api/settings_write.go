package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// settingsUpdate is what the settings page may change.
//
// Three things, exactly as the founding spec allows: the folders to watch, the
// port, and a TMDB key of one's own. Every field is a pointer so that "not
// mentioned" and "set to empty" are different requests -- clearing the API key
// has to be possible, and it must not happen by accident on a form that only
// meant to change the port.
type settingsUpdate struct {
	LibraryPaths *[]string `json:"library_paths"`
	Port         *int      `json:"port"`
	TMDBAPIKey   *string   `json:"tmdb_api_key"`
}

type settingsUpdateResult struct {
	Saved bool `json:"saved"`

	// PortChanged means the new value is on disk but the server is still
	// listening on the old one. Said out loud, because a settings page that
	// silently does nothing is worse than one that refuses.
	PortChanged bool `json:"port_changed"`

	// MissingPaths are directories that were saved but do not currently exist.
	// Saved anyway: an unplugged drive is a normal thing to configure ahead of
	// time, and refusing would be wrong.
	MissingPaths []string `json:"missing_paths,omitempty"`
}

// handleUpdateSettings writes the three configurable values.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsUpdate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid settings payload")
		return
	}

	result := settingsUpdateResult{Saved: true}

	if body.Port != nil {
		port := *body.Port
		if port < 1 || port > 65535 {
			writeJSONError(w, http.StatusBadRequest, "the port must be between 1 and 65535")
			return
		}
		if port != s.cfg.Port {
			result.PortChanged = true
		}
		s.cfg.Port = port
	}

	if body.LibraryPaths != nil {
		cleaned := make([]string, 0, len(*body.LibraryPaths))
		seen := map[string]bool{}
		for _, raw := range *body.LibraryPaths {
			path := strings.TrimSpace(raw)
			if path == "" {
				continue
			}
			if absolute, err := filepath.Abs(path); err == nil {
				path = absolute
			}
			// The same folder twice would scan it twice and change nothing.
			if seen[path] {
				continue
			}
			seen[path] = true
			cleaned = append(cleaned, path)

			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				result.MissingPaths = append(result.MissingPaths, path)
			}
		}
		s.cfg.LibraryPaths = cleaned
	}

	if body.TMDBAPIKey != nil {
		s.cfg.TMDBAPIKey = strings.TrimSpace(*body.TMDBAPIKey)
	}

	if err := s.cfg.Save(); err != nil {
		s.log.Error("saving the settings failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "the settings could not be saved")
		return
	}

	s.log.Info("settings updated", "config", s.cfg)
	writeJSON(w, http.StatusOK, result)
}
