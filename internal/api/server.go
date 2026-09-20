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
	"strconv"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/activity"
	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/imagecache"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/playback"
	"github.com/Benitoow/theia-media/internal/preview"
	"github.com/Benitoow/theia-media/internal/profiles"
	"github.com/Benitoow/theia-media/internal/remoteaccess"
	"github.com/Benitoow/theia-media/internal/supportlog"
	"github.com/Benitoow/theia-media/internal/updater"
	"github.com/Benitoow/theia-media/internal/workload"
)

// Options is everything the server needs. A struct rather than a parameter
// list because this has grown once per milestone and will grow again.
type Options struct {
	Config   *config.Config
	Library  *library.Service
	Images   *imagecache.Cache
	FFmpeg   *ffmpeg.Manager
	State    *db.State
	Updater  *updater.Updater
	Activity *activity.Tracker
	Profiles *profiles.Store
	Remote   *remoteaccess.Service

	// Watcher keeps the library in step with the disk. It owns the list of
	// watched folders, so the settings handler tells it when that list changes.
	// Nil in tests that do not care, and every use of it is guarded.
	Watcher *library.Watcher

	// Previews builds the frames shown under a dragged seek bar. Optional
	// everywhere: nil simply means no previews.
	Previews *preview.Manager
	// SupportLogs is the bounded on-disk history bundled by the explicit support
	// export. Nil keeps tests and read-only embeddings fully functional.
	SupportLogs *supportlog.Store
	Workload    *workload.Coordinator

	// Playback owns the converted-stream execution. Nil builds the default
	// service from FFmpeg and Workload; tests inject one backed by a fake
	// binary, because the real Manager would download a real one.
	Playback *playback.Service

	Web       fs.FS
	Version   string
	KeySource config.KeySource
	Logger    *slog.Logger
}

// Server wires the configuration, the embedded frontend and the JSON API into
// a single http.Handler.
type Server struct {
	settingsMu  sync.Mutex
	savedConfig *config.Config
	cfg         *config.Config
	lib         *library.Service
	images      *imagecache.Cache
	ffmpeg      *ffmpeg.Manager
	state       *db.State
	updater     *updater.Updater
	activity    *activity.Tracker
	profiles    *profiles.Store
	remote      *remoteaccess.Service
	watcher     *library.Watcher
	previews    *preview.Manager
	supportLogs *supportlog.Store
	workload    *workload.Coordinator
	playback    *playback.Service
	web         fs.FS
	log         *slog.Logger
	version     string
	keySource   config.KeySource
	started     time.Time
}

// New builds a Server. Web is the compiled frontend, normally the embedded
// bundle returned by theia.WebFS.
func New(opts Options) *Server {
	// The nil check keeps a nil *workload.Coordinator from becoming a non-nil
	// interface whose first method call panics.
	var workload playback.Workload
	if opts.Workload != nil {
		workload = opts.Workload
	}
	playbackService := opts.Playback
	if playbackService == nil {
		playbackService = playback.NewService(playback.Options{
			Binary:   opts.FFmpeg,
			Workload: workload,
			Logger:   opts.Logger,
		})
	}
	return &Server{
		cfg:         opts.Config,
		lib:         opts.Library,
		images:      opts.Images,
		ffmpeg:      opts.FFmpeg,
		state:       opts.State,
		updater:     opts.Updater,
		activity:    opts.Activity,
		profiles:    opts.Profiles,
		remote:      opts.Remote,
		watcher:     opts.Watcher,
		previews:    opts.Previews,
		supportLogs: opts.SupportLogs,
		workload:    opts.Workload,
		playback:    playbackService,
		web:         opts.Web,
		log:         opts.Logger,
		version:     opts.Version,
		keySource:   opts.KeySource,
		started:     time.Now(),
	}
}

// KillStreams terminates every converted stream this server is feeding.
//
// main calls it on every path that ends the process: killing before the HTTP
// drain lets a film's handler finish instead of holding the shutdown open for
// the length of the film, and killing before an os.Exit is the net that keeps
// the updater's restart from leaving an encoder running beside a dead server.
func (s *Server) KillStreams() {
	s.playback.KillStreams()
}

// Handler returns the fully routed HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/playback/heartbeat", s.handlePlaybackHeartbeat)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("GET /api/diagnostics", s.handleDiagnostics)
	mux.HandleFunc("POST /api/diagnostics/events", s.handleClientDiagnostic)
	mux.HandleFunc("POST /api/diagnostics/export", s.handleSupportExport)
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
	mux.HandleFunc("GET /api/library/watchlist", s.handleWatchlist)
	mux.HandleFunc("PUT /api/library/movies/{id}/watchlist", s.handleSetWatchlist)
	mux.HandleFunc("DELETE /api/library/movies/{id}/watchlist", s.handleSetWatchlist)
	mux.HandleFunc("GET /api/library/movies", s.handleMovies)
	mux.HandleFunc("GET /api/library/movies/{id}", s.handleMovie)
	mux.HandleFunc("POST /api/library/movies/{id}/files/{file_id}/inspect", s.handleInspectMovieFile)
	mux.HandleFunc("GET /api/library/movies/{id}/files/{file_id}/subtitles/{track_id}", s.handleMovieFileSubtitle)
	mux.HandleFunc("PUT /api/library/movies/{id}/progress", s.handleSaveProgress)
	mux.HandleFunc("DELETE /api/library/movies/{id}/progress", s.handleResetProgress)
	// Marking unwatched is exactly forgetting the position, so it is the same
	// handler under the name the client means when it asks.
	mux.HandleFunc("PUT /api/library/movies/{id}/watched", s.handleSetWatched)
	mux.HandleFunc("DELETE /api/library/movies/{id}/watched", s.handleResetProgress)
	mux.HandleFunc("GET /api/library/series", s.handleSeriesList)
	mux.HandleFunc("GET /api/library/series/home", s.handleSeriesHome)
	mux.HandleFunc("GET /api/library/series/{id}", s.handleSeries)
	mux.HandleFunc("GET /api/library/series/{id}/seasons/{season}", s.handleSeason)
	mux.HandleFunc("GET /api/library/episodes/{id}", s.handleEpisode)
	mux.HandleFunc("POST /api/library/episodes/{id}/files/{file_id}/inspect", s.handleInspectEpisodeFile)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/subtitles/{track_id}", s.handleEpisodeFileSubtitle)
	mux.HandleFunc("PUT /api/library/episodes/{id}/progress", s.handleSaveEpisodeProgress)
	mux.HandleFunc("DELETE /api/library/episodes/{id}/progress", s.handleResetEpisodeProgress)
	mux.HandleFunc("PUT /api/library/episodes/{id}/watched", s.handleSetEpisodeWatched)
	mux.HandleFunc("DELETE /api/library/episodes/{id}/watched", s.handleResetEpisodeProgress)
	mux.HandleFunc("GET /api/library/search", s.handleSearch)
	// Correcting a mismatch. LAN only: it changes what a file *is*, for
	// everybody, and remoteRouteAllowed refuses these paths.
	mux.HandleFunc("GET /api/library/movies/{id}/match/candidates", s.handleMovieCandidates)
	mux.HandleFunc("PUT /api/library/movies/{id}/match", s.handleSetMovieMatch)
	mux.HandleFunc("DELETE /api/library/movies/{id}/match", s.handleClearMovieMatch)
	mux.HandleFunc("GET /api/library/series/{id}/match/candidates", s.handleSeriesCandidates)
	mux.HandleFunc("PUT /api/library/series/{id}/match", s.handleSetSeriesMatch)
	mux.HandleFunc("DELETE /api/library/series/{id}/match", s.handleClearSeriesMatch)
	mux.HandleFunc("GET /api/library/stats", s.handleLibraryStats)
	mux.HandleFunc("POST /api/library/scan", s.handleScan)
	mux.HandleFunc("GET /api/images/{size}/{name}", s.handleImage)
	// Seek previews. The sheet is served from one route for every kind of item,
	// because its key is a digest of the file and does not know or care whether
	// that file is a film or an episode.
	mux.HandleFunc("GET /api/previews/{key}", s.handlePreviewSheet)
	mux.HandleFunc("GET /api/previews/{key}/clip", s.handlePreviewClip)
	mux.HandleFunc("GET /api/stream/{id}/preview/clip", s.handleMoviePreviewClip)
	mux.HandleFunc("GET /api/library/episodes/{id}/preview/clip", s.handleEpisodePreviewClip)
	mux.HandleFunc("GET /api/library/series/{id}/preview/clip", s.handleSeriesPreviewClip)
	mux.HandleFunc("GET /api/stream/{id}/preview", s.handleMoviePreview)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/preview", s.handleMovieFilePreview)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/preview", s.handleEpisodeFilePreview)
	mux.HandleFunc("GET /api/stream/{id}/info", s.handleStreamInfo)
	mux.HandleFunc("GET /api/stream/{id}/seek", s.handleLegacySeekStart)
	mux.HandleFunc("GET /api/stream/{id}/remux", s.handleStreamRemux)
	mux.HandleFunc("GET /api/stream/{id}", s.handleStreamDirect)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/info", s.handleMovieFileStreamInfo)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/seek", s.handleMovieFileSeekStart)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}/remux", s.handleMovieFileStreamRemux)
	mux.HandleFunc("GET /api/stream/{id}/files/{file_id}", s.handleMovieFileStreamDirect)
	// Episode streams live below their library resource. Putting them under
	// /api/stream/episodes would overlap the legacy film wildcard routes in Go's
	// ServeMux (some deliberately bizarre IDs can match both patterns).
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/info", s.handleEpisodeFileStreamInfo)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/seek", s.handleEpisodeFileSeekStart)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream/remux", s.handleEpisodeFileStreamRemux)
	mux.HandleFunc("GET /api/library/episodes/{id}/files/{file_id}/stream", s.handleEpisodeFileStreamDirect)
	mux.Handle("/", s.staticHandler())
	// Compression sits inside the log, so a logged status is the one the
	// client actually received.
	return s.logRequests(compress(mux))
}

type healthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`

	// Language is the language this installation was set up in, and it travels
	// with the identity rather than in the settings because both interfaces need
	// it before they draw a word: the OSD reads it from the connection, the
	// browser from this endpoint, and the settings room is behind a LAN-only
	// guard a remote television cannot pass. It is a preference, not a secret.
	Language string `json:"language"`
}

// handleHealth is what the frontend polls to confirm it is talking to a live
// server, and what the updater will use to confirm a restart succeeded.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:        "ok",
		Version:       s.version,
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Language:      s.currentConfig().Language,
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

// writeDeliveryError turns a playback refusal into the response shape the
// interface already knows: a JSON error code, plus Retry-After when the
// refusal says when to come back.
func (s *Server) writeDeliveryError(w http.ResponseWriter, derr *playback.DeliveryError) {
	if derr.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(derr.RetryAfter))
	}
	writeJSONError(w, derr.Status, derr.Code)
}

func (s *Server) beginCostlyWork() func() {
	if s.workload == nil {
		return func() {}
	}
	return s.workload.BeginInteractive()
}
