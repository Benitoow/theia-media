// Command theia is the Theia media server: one binary, no account. It serves
// the embedded web interface on the LAN, can expose a viewer-only interface
// through embedded WireGuard, and announces itself locally through mDNS.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	theia "github.com/Benitoow/theia-media"
	"github.com/Benitoow/theia-media/internal/activity"
	"github.com/Benitoow/theia-media/internal/api"
	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/discovery"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/imagecache"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/preview"
	"github.com/Benitoow/theia-media/internal/profiles"
	"github.com/Benitoow/theia-media/internal/recovery"
	"github.com/Benitoow/theia-media/internal/remoteaccess"
	"github.com/Benitoow/theia-media/internal/supportlog"
	"github.com/Benitoow/theia-media/internal/tmdb"
	"github.com/Benitoow/theia-media/internal/updater"
	"github.com/Benitoow/theia-media/internal/workload"
)

// version is overwritten at build time with -ldflags "-X main.version=v1.2.3".
//
// A build that leaves this as "dev" never updates itself: there is nothing to
// compare a release against, and guessing would overwrite a working binary.
var version = "dev"

// healthExpectationOverride exists only for the update end-to-end harness. An
// official build leaves it empty. The deliberately unhealthy fixture still
// passes `-version` and starts the real server, then fails the same local health
// gate a broken migration or startup would fail, so automatic rollback is
// exercised with actual processes rather than a mocked callback.
var healthExpectationOverride string

// updateRepo is where releases are published and where the updater looks. The
// string itself lives in internal/updater, because the setup tool needs the same
// one to update an installation that is not running.
const updateRepo = updater.DefaultRepo

// tmdbAPIKey is the key official releases ship with, injected by CI from a
// repository secret:
//
//	-ldflags "-X main.tmdbAPIKey=$TMDB_API_KEY"
//
// It is deliberately empty in any build made from a plain `go build`, and it is
// never committed -- this repository is public, and a key in a source file is a
// key published. During development, config.local.json fills the gap.
var tmdbAPIKey string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "theia: %v\n", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	var (
		portFlag    = flag.Int("port", 0, "TCP port to listen on (overrides the configuration file)")
		dataDirFlag = flag.String("data-dir", "", "directory holding the configuration, database and cache")
		verboseFlag = flag.Bool("verbose", false, "log every HTTP request")
		versionFlag = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Println("theia", version)
		return nil
	}

	consoleLevel := slog.LevelInfo
	if *verboseFlag {
		consoleLevel = slog.LevelDebug
	}

	dataDir := *dataDirFlag
	if dataDir == "" {
		var err error
		if dataDir, err = config.DataDir(); err != nil {
			return err
		}
	}
	log, logStore, logErr := supportlog.New(dataDir, os.Stdout, consoleLevel)
	if logErr != nil {
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: consoleLevel}))
		log.Warn("persistent diagnostics are unavailable", "error", logErr)
	} else {
		defer logStore.Close()
	}
	log.Info("theia starting",
		"version", version,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"go_version", runtime.Version(),
		"logical_cpus", runtime.NumCPU(),
		"pid", os.Getpid(),
	)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the running binary: %w", err)
	}
	recoveryManager := recovery.New(dataDir, execPath, version, log)
	// This defer was registered before the database, listener and remote service
	// defers below, so those resources are closed before files are restored.
	defer func() {
		if runErr == nil || !recoveryManager.PendingFor(version) {
			return
		}
		if err := recoveryManager.Rollback(); err != nil {
			log.Error("automatic update rollback failed", "error", err, "previous", execPath+".old")
			return
		}
		restored := exec.Command(execPath, os.Args[1:]...)
		restored.Env = os.Environ()
		restored.Stdout = os.Stdout
		restored.Stderr = os.Stderr
		if err := restored.Start(); err != nil {
			log.Error("the restored version could not be restarted", "error", err)
		}
	}()

	cfg, err := config.Load(dataDir)
	if err != nil {
		return err
	}
	if *portFlag != 0 {
		cfg.Port = *portFlag
	}
	if cfg.HasLocalOverrides() {
		// Worth saying out loud: running with a development port or key without
		// realising it is a confusing afternoon. The key itself is redacted.
		log.Info("config.local.json is in effect", "config", cfg)
	}

	webFS, err := theia.WebFS()
	if err != nil {
		return fmt.Errorf("loading embedded frontend: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(ctx, filepath.Join(dataDir, db.FileName))
	if err != nil {
		return err
	}
	defer database.Close()

	// No key is a supported state, not a failure: the library still scans and
	// browses, it just has no posters. The settings screen says so explicitly
	// rather than leaving the user to guess.
	apiKey, keySource := cfg.ResolveTMDBKey(tmdbAPIKey)
	var tmdbClient *tmdb.Client
	if apiKey != "" {
		// The language this machine was set up in, in TMDB's own vocabulary: the
		// titles and synopses it returns have to agree with the interface that
		// draws them (decision 137).
		tmdbClient = tmdb.New(apiKey, tmdb.WithLanguage(config.MetadataLanguage(cfg.Language)))
		log.Info("TMDB metadata enabled",
			"key_source", keySource, "key", config.Redact(apiKey),
			"language", config.MetadataLanguage(cfg.Language))
	} else {
		// Naming the three places it looked, and the directory the last one is
		// relative to. A locally built binary has no key compiled in and falls
		// back to config.local.json in the *working directory* -- so running the
		// same binary from elsewhere silently produces a library with no
		// artwork, and the message that only said "no key" sent somebody
		// looking for a key that had not moved.
		working, err := os.Getwd()
		if err != nil {
			working = "the working directory"
		}
		log.Warn("no TMDB API key configured, only cached metadata and artwork are available",
			"looked_in", "tmdb_api_key in config.json, the key compiled into this build, config.local.json",
			"working_directory", working)
	}

	store := library.NewStore(database)
	libraryService := library.NewService(store, tmdbClient, log)
	state := db.NewState(database)

	// V2-M1 migrates one-row-per-file installations without changing film ids.
	// Consolidate proven duplicates before the API becomes visible, so an
	// upgraded catalogue never flashes the old duplicate cards while the
	// background scan is starting.
	if merged, err := libraryService.Consolidate(ctx); err != nil {
		return err
	} else if merged > 0 {
		log.Info("consolidated duplicate media records", "merged", merged)
	}

	// Changing the language changes what every synopsis and title should say, and
	// nothing else notices: the cache is keyed by film, not by language. The
	// remembered language and the configured one are compared once, here, before
	// the first scan - matching metadata is marked stale so the enrichment that
	// follows fetches it again in the language this machine was set up in.
	if err := adoptMetadataLanguage(ctx, state, store, config.MetadataLanguage(cfg.Language), log); err != nil {
		log.Warn("could not refresh metadata for the interface language", "error", err)
	}

	// An installation that already has a library has, by definition, already
	// been set up -- somebody pointed it at a folder and watched it scan. The
	// welcome screen exists for a first launch, not for everyone upgrading into
	// the version that added it.
	if err := markOnboardedIfEstablished(ctx, state, libraryService, log); err != nil {
		return err
	}

	images, err := imagecache.New(filepath.Join(dataDir, "cache", "images"), tmdbClient)
	if err != nil {
		return err
	}

	// Nothing is downloaded here. The manager only fetches ffmpeg the first
	// time something actually needs to rewrap a file, which for a library of
	// browser-friendly containers is never.
	transcoder := ffmpeg.New(filepath.Join(dataDir, "bin"), log)
	watching := activity.New()
	jobs := workload.New()

	// Keeps the library in step with the disk on its own. It owns the watched
	// folders from here on: the settings handler hands it the new list, so that
	// this goroutine and an HTTP request are never reading the same slice.
	watcher := library.NewWatcher(libraryService, cfg.LibraryPaths, log)

	// The frames shown under a dragged seek bar. Builds nothing until a player
	// asks, and never when ffmpeg is not already on disk.
	previews, err := preview.NewWithCoordinator(filepath.Join(dataDir, "cache", "previews"), transcoder, log, jobs)
	if err != nil {
		return err
	}
	viewers := profiles.New(database)
	remote := remoteaccess.New(database, dataDir, cfg.Port, log)
	defer remote.Close()

	// Declared here and assigned below, so the restart closure can reach the
	// things it has to release before the replacement binds the port and the
	// mDNS name. os.Exit skips deferred calls, so they are closed explicitly.
	var (
		httpSrv   *http.Server
		announcer *discovery.Announcer
		apiServer *api.Server
	)

	selfUpdater := updater.New(updater.Options{
		Repo:     updateRepo,
		Version:  version,
		ExecPath: execPath,
		Activity: watching,
		Logger:   log,
		// Points the updater at something other than GitHub. This exists so the
		// whole install-and-restart cycle can be exercised against a local stub
		// on a real binary, rather than only in unit tests; it is also the hook
		// anyone mirroring releases internally would need.
		APIBase: os.Getenv("THEIA_UPDATE_API"),
		Prepare: func(ctx context.Context, target string) error {
			return recoveryManager.Prepare(ctx, database, target)
		},
		Abort: recoveryManager.Abort,
		Restart: func() {
			// The response that triggered this is still on its way to the
			// browser; losing it would leave the interface showing "installing"
			// forever.
			time.Sleep(1500 * time.Millisecond)
			log.Info("restarting into the new version")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if httpSrv != nil {
				// Frees the port. A listening socket is released immediately on
				// close, so the replacement can bind straight away.
				_ = httpSrv.Shutdown(shutdownCtx)
			}
			remote.Close()
			_ = announcer.Close()
			// os.Exit skips every deferred call, so the kill is explicit: a
			// replacement starting beside an encoder still feeding a film is
			// the orphan this exists to prevent.
			apiServer.KillStreams()

			replacement := exec.Command(execPath, os.Args[1:]...)
			replacement.Env = os.Environ()
			replacement.Stdout = os.Stdout
			replacement.Stderr = os.Stderr
			if err := replacement.Start(); err != nil {
				log.Error("the new version could not be started; the previous one is beside it",
					"error", err, "previous", execPath+".old")
				os.Exit(1)
			}
			os.Exit(0)
		},
	})

	// Bind before announcing anything: failing here is the one startup error
	// users actually hit, and it should not be preceded by a cheerful banner.
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.Port)))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d (is Theia already running?): %w", cfg.Port, err)
	}
	defer listener.Close()

	apiServer = api.New(api.Options{
		Config:      cfg,
		Library:     libraryService,
		Images:      images,
		FFmpeg:      transcoder,
		State:       state,
		Updater:     selfUpdater,
		Activity:    watching,
		Profiles:    viewers,
		Remote:      remote,
		Watcher:     watcher,
		Previews:    previews,
		SupportLogs: logStore,
		Workload:    jobs,
		Web:         webFS,
		Version:     version,
		KeySource:   keySource,
		Logger:      log,
	})
	apiHandler := apiServer.Handler()
	// The net under every exit: a converted stream outlives neither the
	// process nor the registry that tracks it. The explicit calls below kill
	// before the drains they speed up; this one covers the paths that simply
	// return -- a serve error, a failed health check -- whose handlers may
	// still be feeding a film when the process goes away.
	defer apiServer.KillStreams()
	httpSrv = &http.Server{
		Handler: remoteaccess.LANOnly(apiHandler, cfg.Hostname),
		// Guards against a client that opens a connection and never finishes
		// sending its request headers. There is deliberately no WriteTimeout:
		// video streaming holds a single response open for the length of a film.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	if err := remote.Start(ctx, apiHandler); err != nil {
		// Remote access always fails closed. Keep the LAN listener alive so the
		// owner has a recovery path to change the UDP port or disable the feature.
		log.Warn("remote access is unavailable; LAN access remains active", "error", err)
	}

	announcer, err = discovery.Announce(cfg.Hostname, cfg.Port, version, log)
	if err != nil {
		// Not fatal by design. The QR code and the plain IP address are the
		// reliable ways in; mDNS is the convenience layered on top.
		log.Warn("mDNS unavailable, reach the server by IP address instead", "error", err)
	}
	defer announcer.Close()

	printBanner(cfg)

	serveErr := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	healthVersion := version
	if healthExpectationOverride != "" {
		healthVersion = healthExpectationOverride
	}
	if err := verifyLocalHealth(ctx, cfg.Port, healthVersion); err != nil {
		apiServer.KillStreams()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = httpSrv.Shutdown(shutdownCtx)
		cancel()
		return err
	}
	if recoveryManager.PendingFor(version) {
		if err := recoveryManager.Commit(); err != nil {
			log.Warn("the update is healthy but its recovery point could not be cleared", "error", err)
		} else {
			updater.CleanPrevious(execPath, log)
		}
	} else {
		updater.CleanPrevious(execPath, log)
	}

	// Scan in the background rather than before serving. A library on a slow
	// external drive would otherwise hold the interface hostage at exactly the
	// moment the user is trying to see whether the thing works at all.
	//
	// The watcher performs that first scan and then keeps going, so that a film
	// dropped into a folder appears on its own. Nothing here waits for it.
	go watcher.Run(ctx)

	// Checks only, never installs. Applying is an explicit action, because a
	// media server that restarts itself unannounced is one nobody trusts.
	go selfUpdater.Run(ctx)

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
		// Killing before the drain: a killed encoder closes its pipe, its
		// handler returns, and Shutdown is not left waiting out the rest of a
		// film.
		apiServer.KillStreams()
	}

	remote.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	return nil
}

func verifyLocalHealth(ctx context.Context, port int, wantVersion string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/api/health"
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			var health struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			decodeErr := json.NewDecoder(res.Body).Decode(&health)
			res.Body.Close()
			if res.StatusCode == http.StatusOK && decodeErr == nil && health.Status == "ok" && health.Version == wantVersion {
				statsReq, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/library/stats", nil)
				if requestErr == nil {
					stats, statsErr := client.Do(statsReq)
					if statsErr == nil {
						_, _ = io.Copy(io.Discard, stats.Body)
						stats.Body.Close()
						if stats.StatusCode == http.StatusOK {
							return nil
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("local health verification failed: %w", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// markOnboardedIfEstablished suppresses the welcome screen for an installation
// that clearly predates it: a database with films in it belongs to somebody who
// has already been through setup, whatever version they were on at the time.
// adoptMetadataLanguage notices a change of interface language and marks the
// stored metadata stale in response.
//
// The cache is keyed by film, not by language, so a machine set up in English
// and later switched to French would keep drawing English titles and synopses -
// silently, and for as long as the installation lives. The remembered language
// is compared once, at startup and before the first scan: a mismatch puts every
// fetched record back in the queue the enrichment pass already drains, which is
// cheaper than teaching a second pass how to overwrite.
// legacyMetadataLanguage is what every build before decision 137 asked TMDB for.
// Named here because the upgrade path depends on it, and a reader of this file
// should not have to go looking for it.
const legacyMetadataLanguage = "fr-FR"

func adoptMetadataLanguage(ctx context.Context, state *db.State, store *library.Store, language string, log *slog.Logger) error {
	previous, _, err := state.Get(ctx, db.KeyMetadataLanguage)
	if err != nil {
		return err
	}
	if previous == language {
		return nil
	}
	switch {
	case previous != "":
		if err := store.MarkMetadataStale(ctx); err != nil {
			return err
		}
		log.Info("the interface language changed, refetching metadata", "from", previous, "to", language)
	case language != legacyMetadataLanguage:
		// No marker and this machine does not want French: everything stored was
		// fetched by a build whose TMDB language was the constant "fr-FR", so the
		// library is carrying French titles and synopses whatever this
		// installation is set to now. Measured on the maintainer's own server on
		// 20 September 2026: the interface came back in English and the film
		// titles did not.
		if err := store.MarkMetadataStale(ctx); err != nil {
			return err
		}
		log.Info("refetching metadata left by an older build", "language", language)
	}
	return state.Set(ctx, db.KeyMetadataLanguage, language)
}

func markOnboardedIfEstablished(ctx context.Context, state *db.State,
	lib *library.Service, log *slog.Logger,
) error {
	done, err := state.Has(ctx, db.KeyOnboardingCompleted)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	count, err := lib.Count(ctx)
	if err != nil {
		return err
	}
	series, err := lib.SeriesCount(ctx)
	if err != nil {
		return err
	}
	if count == 0 && series == 0 {
		return nil // a genuine first launch; the welcome screen is for this
	}

	log.Info("existing library found, skipping the welcome screen", "films", count, "series", series)
	return state.Set(ctx, db.KeyOnboardingCompleted, "pre-existing")
}

// printBanner lists every way to reach the server. This goes to stdout rather
// than the logger on purpose: it is the first thing a user reads, and it should
// not be wrapped in timestamps and key=value pairs.
func printBanner(cfg *config.Config) {
	fmt.Printf("\n  Theia %s\n\n", version)
	fmt.Printf("  Local     http://localhost:%d\n", cfg.Port)
	if ip, err := discovery.PreferredAddr(); err == nil {
		fmt.Printf("  Network   http://%s:%d\n", ip, cfg.Port)
	}
	fmt.Printf("  mDNS      http://%s.local:%d\n", cfg.Hostname, cfg.Port)
	fmt.Printf("\n  Data      %s\n\n", cfg.Dir())
}
