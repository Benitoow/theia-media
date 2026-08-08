package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

// V2-M6: re-encoding the picture, and refusing to pretend it is free.
//
// Two rules shape everything here. Nothing is offered that this machine cannot
// actually produce -- the encoder list is a probe, not a compile-time list. And
// nothing is started that would ruin what is already playing: a software
// transcode of a 1080p source runs at about real time, so a second one does not
// run at all, it makes both stall.

// videoQuality is one rung the interface may offer.
type videoQuality struct {
	// Height in pixels, or 0 for the source as it is.
	Height int `json:"height"`

	// Mode is what playing this rung would do: "direct", "remux" or
	// "transcode". The interface uses it to say what a choice costs.
	Mode string `json:"mode"`
}

// transcodeInfo travels on /info so the player can build its menu from facts.
type transcodeInfo struct {
	// Available is false when no encoder on this machine runs. The interface
	// then offers no quality at all rather than a button that fails.
	Available bool `json:"available"`

	// Kind is "hardware" or "software", which is the whole difference between
	// a quality change being free and being a decision.
	Kind string `json:"kind,omitempty"`

	// Encoder is for the log and a support question, never for a sentence.
	Encoder string `json:"encoder,omitempty"`

	// Busy says every transcoding slot is taken, so the interface can grey the
	// rungs that would need one instead of letting a press fail.
	Busy bool `json:"busy,omitempty"`
}

// transcodeLimiter keeps the furnace from being lit twice.
//
// The numbers come from a measurement rather than a feeling: on the
// maintainer's machine a 1080p HEVC source re-encodes at 1.04x real time in
// software and 4.56x on the GPU. One software transcode therefore consumes the
// whole margin, and a second would leave both viewers watching a spinner --
// the failure mode where nobody can tell what went wrong. Hardware has room
// for a few.
type transcodeLimiter struct {
	mu     sync.Mutex
	active int
	limit  int
}

func newTranscodeLimiter() *transcodeLimiter {
	return &transcodeLimiter{limit: 1}
}

// setKind raises the ceiling once the encoder is known.
func (l *transcodeLimiter) setKind(kind ffmpeg.EncoderKind) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if kind == ffmpeg.KindHardware {
		l.limit = 3
	} else {
		l.limit = 1
	}
}

// acquire takes a slot, returning the release function and whether it got one.
func (l *transcodeLimiter) acquire() (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active >= l.limit {
		return nil, false
	}
	l.active++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.active--
	}, true
}

func (l *transcodeLimiter) busy() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active >= l.limit
}

// forcedTranscode reads ?video=transcode.
//
// This is how a browser reports what no server can know. `hevc` is classified
// risky rather than unsupported because Safari plays it and Chrome does not, so
// the remux is worth attempting -- and when it fails it fails silently, with
// sound over a picture that never arrives. The player detects that (videoWidth
// stays 0) and asks again with this flag, which turns the one dead end M1 could
// not resolve into a film that plays.
func forcedTranscode(r *http.Request) bool {
	return r.URL.Query().Get("video") == "transcode"
}

// requestedHeight reads ?h=. Absent means "leave the picture alone".
func requestedHeight(w http.ResponseWriter, r *http.Request) (int, bool, bool) {
	raw, present := r.URL.Query()["h"]
	if !present {
		return 0, false, true
	}
	if len(raw) != 1 {
		writeJSONError(w, http.StatusBadRequest, "invalid_height")
		return 0, false, false
	}
	height, err := strconv.Atoi(raw[0])
	if err != nil || height <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_height")
		return 0, false, false
	}
	// Only a rung this server actually offers. An arbitrary number would let a
	// caller ask for a scale nobody sized a bitrate for.
	for _, offered := range stream.Qualities {
		if offered == height {
			return height, true, true
		}
	}
	writeJSONError(w, http.StatusBadRequest, "invalid_height")
	return 0, false, false
}

// videoCapabilities answers what this machine can do.
//
// The M1 rule it has to respect is precise: asking how a file will play must
// never *download* ffmpeg. Probing an ffmpeg that is already on disk is a
// different thing -- five subprocesses run concurrently, once per process --
// so Available() is the gate rather than a blanket refusal. A machine that has
// never needed ffmpeg reports no quality menu, which is true: it cannot encode
// anything until the first remux fetches the binary.
func (s *Server) videoCapabilities(ctx context.Context) transcodeInfo {
	if s.ffmpeg == nil || !ffmpeg.Supported() || !s.ffmpeg.Available() {
		return transcodeInfo{}
	}

	caps := s.ffmpeg.Capabilities(ctx)
	best, ok := caps.Best()
	if !ok {
		return transcodeInfo{}
	}
	s.transcodes.setKind(best.Kind)
	return transcodeInfo{
		Available: true,
		Kind:      string(best.Kind),
		Encoder:   best.Name,
		Busy:      s.transcodes.busy(),
	}
}

// qualityLadder is what the interface may offer for one file.
//
// "Original" is always first and is whatever the file already does -- direct
// play, or a remux. The rungs below it exist only when something can encode
// them, and never above the source.
func qualityLadder(media library.FileMedia, base stream.Decision, canTranscode bool) []videoQuality {
	original := videoQuality{Height: 0, Mode: string(base.Mode)}
	if base.Mode == stream.ModeUnsupported && canTranscode {
		// The case that opened this milestone: an HEVC file no browser decodes
		// becomes playable, at its own size, by being re-encoded.
		original.Mode = string(stream.ModeTranscode)
	}
	ladder := []videoQuality{original}

	if !canTranscode || media.Status != library.MediaOK || media.Video == nil {
		return ladder
	}
	for _, height := range stream.AvailableHeights(media.Video.Height) {
		ladder = append(ladder, videoQuality{Height: height, Mode: string(stream.ModeTranscode)})
	}
	return ladder
}
