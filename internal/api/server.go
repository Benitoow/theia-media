// Package api exposes Theia's HTTP surface: the small JSON API the frontend
// talks to, and the embedded single-page app itself.
//
// There is no authentication anywhere in this package, and that is a deliberate
// scope decision for v1 rather than an oversight -- Theia assumes it is running
// on a trusted LAN. See the warning in README.md.
package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/library"
)

// Server wires the configuration, the embedded frontend and the JSON API into
// a single http.Handler.
type Server struct {
	cfg     *config.Config
	lib     *library.Service
	web     fs.FS
	log     *slog.Logger
	version string
	started time.Time
}

// New builds a Server. web is the compiled frontend, normally the embedded
// bundle returned by theia.WebFS.
func New(cfg *config.Config, lib *library.Service, web fs.FS, version string, log *slog.Logger) *Server {
	return &Server{
		cfg:     cfg,
		lib:     lib,
		web:     web,
		log:     log,
		version: version,
		started: time.Now(),
	}
}

// Handler returns the fully routed HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/library/movies", s.handleMovies)
	mux.HandleFunc("GET /api/library/stats", s.handleLibraryStats)
	mux.HandleFunc("POST /api/library/scan", s.handleScan)
	mux.Handle("/", s.staticHandler())
	return s.logRequests(mux)
}

type healthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// handleHealth is what the frontend polls to confirm it is talking to a live
// server, and what the updater will use to confirm a restart succeeded.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:        "ok",
		Version:       s.version,
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
	})
}

// logRequests records one line per request, with the status code the handler
// actually wrote.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		s.log.Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		)
	})
}

// statusRecorder remembers the status code on its way through.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the underlying ResponseWriter, so
// that wrapping here does not cost us Flush or SetWriteDeadline. Streaming in
// M4 depends on both.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// The status line is already on the wire, so there is nothing useful
		// left to say to the client.
		return
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
