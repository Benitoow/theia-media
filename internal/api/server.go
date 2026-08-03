// Package api exposes Theia's HTTP surface: the small JSON API the frontend
// talks to, and the embedded single-page app itself.
//
// LAN administration deliberately keeps the v1 zero-authentication model. The
// separate remote listener authenticates WireGuard devices and exposes only a
// viewer-safe subset of these routes. See internal/remoteaccess and README.md.
package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/Benitoow/theia-media/internal/activity"
	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/imagecache"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/profiles"
	"github.com/Benitoow/theia-media/internal/remoteaccess"
	"github.com/Benitoow/theia-media/internal/updater"
)

// Options is everything the server needs. A struct rather than a parameter
// list because this has grown once per milestone and will grow again.
type Options struct {
	Config    *config.Config
	Library   *library.Service
	Images    *imagecache.Cache
	FFmpeg    *ffmpeg.Manager
	State     *db.State
	Updater   *updater.Updater
	Activity  *activity.Tracker
	Profiles  *profiles.Store
	Remote    *remoteaccess.Service
	Web       fs.FS
	Version   string
	KeySource config.KeySource
	Logger    *slog.Logger
}

// Server wires the configuration, the embedded frontend and the JSON API into
// a single http.Handler.
type Server struct {
	cfg       *config.Config
	lib       *library.Service
	images    *imagecache.Cache
	ffmpeg    *ffmpeg.Manager
	state     *db.State
	updater   *updater.Updater
	activity  *activity.Tracker
	profiles  *profiles.Store
	remote    *remoteaccess.Service
	web       fs.FS
	log       *slog.Logger
	version   string
	keySource config.KeySource
	started   time.Time
}

// New builds a Server. Web is the compiled frontend, normally the embedded
// bundle returned by theia.WebFS.
func New(opts Options) *Server {
	return &Server{
		cfg:       opts.Config,
		lib:       opts.Library,
		images:    opts.Images,
		ffmpeg:    opts.FFmpeg,
		state:     opts.State,
		updater:   opts.Updater,
		activity:  opts.Activity,
		profiles:  opts.Profiles,
		remote:    opts.Remote,
		web:       opts.Web,
		log:       opts.Logger,
		version:   opts.Version,
		keySource: opts.KeySource,
		started:   time.Now(),
	}
}

// Handler returns the fully routed HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)
	mux.HandleFunc("GET /api/onboarding", s.handleOnboarding)
	mux.HandleFunc("POST /api/onboarding/complete", s.handleCompleteOnboarding)
	mux.HandleFunc("GET /api/update", s.handleUpdateStatus)
	mux.HandleFunc("POST /api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("GET /api/remote-access", s.handleRemoteAccessStatus)
	mux.HandleFunc("PUT /api/remote-access", s.handleUpdateRemoteAccess)
	mux.HandleFunc("POST /api/remote-access/peers", s.handleCreateRemotePeer)
	mux.HandleFunc("DELETE /api/remote-access/peers/{id}", s.handleRevokeRemotePeer)
	mux.HandleFunc("GET /api/remote-access/session", s.handleRemoteSession)
	mux.HandleFunc("GET /api/profiles", s.handleProfiles)
	mux.HandleFunc("POST /api/profiles", s.handleCreateProfile)
	mux.HandleFunc("GET /api/profiles/{id}", s.handleProfile)
	mux.HandleFunc("PATCH /api/profiles/{id}", s.handleRenameProfile)
	mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
	mux.HandleFunc("GET /api/profiles/{id}/avatar", s.handleProfileAvatar)
	mux.HandleFunc("PUT /api/profiles/{id}/avatar", s.handleSetProfileAvatar)
	mux.HandleFunc("DELETE /api/profiles/{id}/avatar", s.handleDeleteProfileAvatar)
	mux.HandleFunc("GET /api/library/home", s.handleHome)
	mux.HandleFunc("GET /api/library/movies", s.handleMovies)
	mux.HandleFunc("GET /api/library/movies/{id}", s.handleMovie)
	mux.HandleFunc("POST /api/library/movies/{id}/files/{file_id}/inspect", s.handleInspectMovieFile)
	mux.HandleFunc("PUT /api/library/movies/{id}/progress", s.handleSaveProgress)
	mux.HandleFunc("DELETE /api/library/movies/{id}/progress", s.handleResetProgress)
	mux.HandleFunc("GET /api/library/series", s.handleSeriesList)
	mux.HandleFunc("GET /api/library/series/home", s.handleSeriesHome)
	mux.HandleFunc("GET /api/library/series/{id}", s.handleSeries)
	mux.HandleFunc("GET /api/library/series/{id}/seasons/{season}", s.handleSeason)
	mux.HandleFunc("GET /api/library/episodes/{id}", s.handleEpisode)
	mux.HandleFunc("POST /api/library/episodes/{id}/files/{file_id}/inspect", s.handleInspectEpisodeFile)
	mux.HandleFunc("PUT /api/library/episodes/{id}/progress", s.handleSaveEpisodeProgress)
	mux.HandleFunc("DELETE /api/library/episodes/{id}/progress", s.handleResetEpisodeProgress)
	mux.HandleFunc("GET /api/library/stats", s.handleLibraryStats)
	mux.HandleFunc("POST /api/library/scan", s.handleScan)
	mux.HandleFunc("GET /api/images/{size}/{name}", s.handleImage)
	mux.HandleFunc("GET /api/stream/{id}/info", s.handleStreamInfo)
	mux.HandleFunc("GET /api/stream/{id}/remux", s.handleStreamRemux)
	mux.HandleFunc("GET /api/stream/{id}", s.handleStreamDirect)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/info", s.handleMovieFileStreamInfo)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/remux", s.handleMovieFileStreamRemux)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}", s.handleMovieFileStreamDirect)
	// Episode streams live below their library resource. Putting them under
	// /api/stream/episodes would overlap the legacy film wildcard routes in Go's
	// ServeMux (some deliberately bizarre IDs can match both patterns).
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/info", s.handleEpisodeFileStreamInfo)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/remux", s.handleEpisodeFileStreamRemux)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream", s.handleEpisodeFileStreamDirect)
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
