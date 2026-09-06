package api

import "net/http"

func (s *Server) beginPlayback(w http.ResponseWriter) (func(), bool) {
	if s.activity == nil {
		return func() {}, true
	}
	end, ok := s.activity.TryBegin()
	if !ok {
		writeJSONError(w, http.StatusServiceUnavailable, "update_restarting")
	}
	return end, ok
}

// A buffered direct file may generate no Range requests for minutes. The
// visible player renews this lease so an idle connection is not an idle viewer.
func (s *Server) handlePlaybackHeartbeat(w http.ResponseWriter, r *http.Request) {
	end, ok := s.beginPlayback(w)
	if !ok {
		return
	}
	end()
	w.WriteHeader(http.StatusNoContent)
}
