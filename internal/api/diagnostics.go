package api

import (
	"net/http"
	"time"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
)

// Everything this file reports is something Theia already knew and never said.
//
// Decisions 58, 59 and 60 measure what this machine can do — which encoders
// answer when asked to produce a frame, whether a hardware decoder exists, what
// the software fallback costs — and then use those measurements silently. The
// project's own standard is to report what was verified rather than what was
// assumed; that is easier to hold to when the verification is on a page.
//
// Nothing here probes anything that is not already on disk. Asking what this
// machine can do must never be what causes it to download ffmpeg.

type diagnosticsResponse struct {
	FFmpeg  ffmpegDiagnostics  `json:"ffmpeg"`
	Library libraryDiagnostics `json:"library"`
	Images  imageDiagnostics   `json:"images"`
	DataDir string             `json:"data_dir"`
}

type ffmpegDiagnostics struct {
	// Supported is whether this OS and architecture have a pinned build at all.
	Supported bool `json:"supported"`

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
	out := diagnosticsResponse{DataDir: s.cfg.Dir()}

	out.FFmpeg.Supported = ffmpeg.Supported()
	if s.ffmpeg != nil {
		out.FFmpeg.Present = s.ffmpeg.Available()
	}
	if out.FFmpeg.Supported && out.FFmpeg.Present {
		// Already on disk, so this is subprocesses rather than a download. The
		// result is computed once per process and remembered.
		caps := s.ffmpeg.Capabilities(r.Context())
		out.FFmpeg.Probed = true
		out.FFmpeg.Encoders = caps.Encoders
		out.FFmpeg.HardwareDecoder = s.ffmpeg.HardwareDecoder(r.Context())
	}
	if out.FFmpeg.Encoders == nil {
		out.FFmpeg.Encoders = []ffmpeg.Encoder{}
	}

	if s.lib != nil {
		out.Library.Films, _ = s.lib.Count(r.Context())
		out.Library.Series, _ = s.lib.SeriesCount(r.Context())
		out.Library.Episodes, _ = s.lib.EpisodeCount(r.Context())
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

	writeJSON(w, http.StatusOK, out)
}
