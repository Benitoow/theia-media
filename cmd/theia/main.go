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
	"strconv"
	"syscall"
	"time"

	theia "github.com/Benitoow/theia-media"
	"github.com/Benitoow/theia-media/internal/api"
	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/discovery"
)

// version is overwritten at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

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

	webFS, err := theia.WebFS()
	if err != nil {
		return fmt.Errorf("loading embedded frontend: %w", err)
	}

	// Bind before announcing anything: failing here is the one startup error
	// users actually hit, and it should not be preceded by a cheerful banner.
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.Port)))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d (is Theia already running?): %w", cfg.Port, err)
	}

	httpSrv := &http.Server{
		Handler: api.New(cfg, webFS, version, log).Handler(),
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
