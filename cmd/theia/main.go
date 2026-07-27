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
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	theia "github.com/Benitoow/theia-media"
	"github.com/Benitoow/theia-media/internal/api"
	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/discovery"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/imagecache"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/tmdb"
)

// version is overwritten at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

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

	images, err := imagecache.New(filepath.Join(dataDir, "cache", "images"), tmdbClient)
	if err != nil {
		return err
	}

	// Nothing is downloaded here. The manager only fetches ffmpeg the first
	// time something actually needs to rewrap a file, which for a library of
	// browser-friendly containers is never.
	transcoder := ffmpeg.New(filepath.Join(dataDir, "bin"), log)

	// Bind before announcing anything: failing here is the one startup error
	// users actually hit, and it should not be preceded by a cheerful banner.
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.Port)))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d (is Theia already running?): %w", cfg.Port, err)
	}

	httpSrv := &http.Server{
		Handler: api.New(api.Options{
			Config:    cfg,
			Library:   libraryService,
			Images:    images,
			FFmpeg:    transcoder,
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

	announcer, err := discovery.Announce(cfg.Hostname, cfg.Port, log)
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
