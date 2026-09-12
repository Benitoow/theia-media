package api

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/supportlog"
	"github.com/Benitoow/theia-media/internal/updater"
)

// Everything this file reports is something Theia already knew and never said.
//
// Decisions 58, 59 and 60 measure what this machine can do - which encoders
// answer when asked to produce a frame, whether a hardware decoder exists, what
// the software fallback costs - and then use those measurements silently. The
// project's own standard is to report what was verified rather than what was
// assumed; that is easier to hold to when the verification is on a page.
//
// Nothing here probes anything that is not already on disk. Asking what this
// machine can do must never be what causes it to download ffmpeg.

type diagnosticsResponse struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Version     string             `json:"version"`
	System      systemDiagnostics  `json:"system"`
	Process     processDiagnostics `json:"process"`
	FFmpeg      ffmpegDiagnostics  `json:"ffmpeg"`
	Library     libraryDiagnostics `json:"library"`
	Images      imageDiagnostics   `json:"images"`
	Logs        supportlog.Stats   `json:"logs"`
	Update      *updater.Status    `json:"update,omitempty"`
	DataDir     string             `json:"data_dir"`
}

type systemDiagnostics struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	GoVersion   string `json:"go_version"`
	LogicalCPUs int    `json:"logical_cpus"`
}

type processDiagnostics struct {
	UptimeSeconds    int64  `json:"uptime_seconds"`
	Goroutines       int    `json:"goroutines"`
	MemoryAllocBytes uint64 `json:"memory_alloc_bytes"`
	MemorySysBytes   uint64 `json:"memory_sys_bytes"`
}

type ffmpegDiagnostics struct {
	// Supported is whether this OS and architecture have a pinned build at all.
	Supported bool                    `json:"supported"`
	Runtime   *ffmpeg.RuntimeManifest `json:"runtime,omitempty"`

	// Present is whether it has been downloaded yet. False is an ordinary
	// state, not a fault: a library of browser-friendly files never needs it.
	Present bool `json:"present"`

	// Probed says whether the encoder list below was measured. It is false when
	// ffmpeg is not on disk, because finding out would mean fetching it.
	Probed bool `json:"probed"`

	// Encoders are the ones that answered, best first. An empty list on a
	// probed machine means software encoding only.
	Encoders []ffmpeg.Encoder `json:"encoders"`

	// HardwareDecoder is the accelerator decoding will use, or empty for none.
	HardwareDecoder string `json:"hardware_decoder,omitempty"`
}

type libraryDiagnostics struct {
	Films    int `json:"films"`
	Series   int `json:"series"`
	Episodes int `json:"episodes"`

	// Watching is the folder list the watcher is actually using, which is the
	// configuration unless somebody has just changed it.
	Watching []string `json:"watching"`

	// WatchIntervalSeconds is how often the folders are looked at. Zero when
	// nothing is watching them, which is how a build without a watcher reads.
	WatchIntervalSeconds int `json:"watch_interval_seconds"`

	Scanning bool                `json:"scanning"`
	LastScan *library.ScanReport `json:"last_scan,omitempty"`
}

type imageDiagnostics struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// handleDiagnostics reports what this installation is and what it can do.
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.collectDiagnostics(r.Context()))
}

func (s *Server) collectDiagnostics(ctx context.Context) diagnosticsResponse {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	out := diagnosticsResponse{
		GeneratedAt: time.Now().UTC(),
		Version:     s.version,
		DataDir:     s.cfg.Dir(),
		System: systemDiagnostics{
			OS: runtime.GOOS, Arch: runtime.GOARCH,
			GoVersion: runtime.Version(), LogicalCPUs: runtime.NumCPU(),
		},
		Process: processDiagnostics{
			UptimeSeconds:    int64(time.Since(s.started).Seconds()),
			Goroutines:       runtime.NumGoroutine(),
			MemoryAllocBytes: memory.Alloc,
			MemorySysBytes:   memory.Sys,
		},
	}

	out.FFmpeg.Supported = ffmpeg.Supported()
	if manifest, ok := ffmpeg.Manifest(); ok {
		out.FFmpeg.Runtime = &manifest
	}
	if s.ffmpeg != nil {
		out.FFmpeg.Present = s.ffmpeg.Available()
	}
	if out.FFmpeg.Supported && out.FFmpeg.Present {
		// Already on disk, so this is subprocesses rather than a download. The
		// result is computed once per process and remembered.
		caps := s.ffmpeg.Capabilities(ctx)
		out.FFmpeg.Probed = true
		out.FFmpeg.Encoders = caps.Encoders
		out.FFmpeg.HardwareDecoder = s.ffmpeg.HardwareDecoder(ctx)
	}
	if out.FFmpeg.Encoders == nil {
		out.FFmpeg.Encoders = []ffmpeg.Encoder{}
	}

	if s.lib != nil {
		out.Library.Films, _ = s.lib.Count(ctx)
		out.Library.Series, _ = s.lib.SeriesCount(ctx)
		out.Library.Episodes, _ = s.lib.EpisodeCount(ctx)
		out.Library.Scanning = s.lib.Scanning()
		out.Library.LastScan = s.lib.LastScan()
	}
	out.Library.Watching = s.libraryRoots()
	if out.Library.Watching == nil {
		out.Library.Watching = []string{}
	}
	if s.watcher != nil {
		out.Library.WatchIntervalSeconds = int(library.DefaultWatchInterval / time.Second)
	}

	if s.images != nil {
		files, bytes, err := s.images.Usage()
		if err != nil {
			s.log.Warn("could not measure the image cache", "error", err)
		}
		out.Images = imageDiagnostics{Files: files, Bytes: bytes}
	}
	if s.supportLogs != nil {
		out.Logs = s.supportLogs.Usage()
	}
	if s.updater != nil {
		status := s.updater.Status()
		out.Update = &status
	}
	return out
}
