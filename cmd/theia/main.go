// Command theia is the Theia media server: one binary, no configuration, no
// account. It serves the embedded web interface over HTTP and announces itself
// on the local network so other devices can find it without an IP address.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
	"github.com/Benitoow/theia-media/internal/tmdb"
	"github.com/Benitoow/theia-media/internal/updater"
)

// version is overwritten at build time with -ldflags "-X main.version=v1.2.3".
//
// A build that leaves this as "dev" never updates itself: there is nothing to
// compare a release against, and guessing would overwrite a working binary.
var version = "dev"

// updateRepo is where releases are published and where the updater looks.
const updateRepo = "Benitoow/theia-media"

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

func run() error {
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

	level := slog.LevelInfo
	if *verboseFlag {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	dataDir := *dataDirFlag
	if dataDir == "" {
		var err error
		if dataDir, err = config.DataDir(); err != nil {
			return err
		}
	}

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
		tmdbClient = tmdb.New(apiKey)
		log.Info("TMDB metadata enabled", "key_source", keySource, "key", config.Redact(apiKey))
	} else {
		log.Warn("no TMDB API key configured, films will be listed without metadata")
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
		log.Info("consolidated duplicate film records", "merged", merged)
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

	// Where this binary lives, and the leftover from any previous update. The
	// outgoing executable cannot be deleted while it is running, so it is
	// cleared away on the next start instead.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the running binary: %w", err)
	}
	updater.CleanPrevious(execPath, log)

	// Declared here and assigned below, so the restart closure can reach the
	// things it has to release before the replacement binds the port and the
	// mDNS name. os.Exit skips deferred calls, so they are closed explicitly.
	var (
		httpSrv   *http.Server
		announcer *discovery.Announcer
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
			_ = announcer.Close()

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

	httpSrv = &http.Server{
		Handler: api.New(api.Options{
			Config:    cfg,
			Library:   libraryService,
			Images:    images,
			FFmpeg:    transcoder,
			State:     state,
			Updater:   selfUpdater,
			Activity:  watching,
			Web:       webFS,
			Version:   version,
			KeySource: keySource,
			Logger:    log,
		}).Handler(),
		// Guards against a client that opens a connection and never finishes
		// sending its request headers. There is deliberately no WriteTimeout:
		// video streaming holds a single response open for the length of a film.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	announcer, err = discovery.Announce(cfg.Hostname, cfg.Port, log)
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

	// Scan in the background rather than before serving. A library on a slow
	// external drive would otherwise hold the interface hostage at exactly the
	// moment the user is trying to see whether the thing works at all.
	go func() {
		if _, err := libraryService.Scan(ctx, cfg.LibraryPaths); err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Error("the initial scan failed", "error", err)
		}
	}()

	// Checks only, never installs. Applying is an explicit action, because a
	// media server that restarts itself unannounced is one nobody trusts.
	go selfUpdater.Run(ctx)

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	return nil
}

// markOnboardedIfEstablished suppresses the welcome screen for an installation
// that clearly predates it: a database with films in it belongs to somebody who
// has already been through setup, whatever version they were on at the time.
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
	if count == 0 {
		return nil // a genuine first launch; the welcome screen is for this
	}

	log.Info("existing library found, skipping the welcome screen", "films", count)
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
