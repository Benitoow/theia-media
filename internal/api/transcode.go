package api

import (
	"context"

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
// run at all, it makes both stall. The limiter that enforces the second rule
// lives with the execution it guards, in internal/playback.

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
	kind, encoder, busy, ok := s.playback.TranscodeStatus(ctx)
	if !ok {
		return transcodeInfo{}
	}
	return transcodeInfo{
		Available: true,
		Kind:      kind,
		Encoder:   encoder,
		Busy:      busy,
	}
}

// qualityLadder is what the interface may offer for one file.
//
// "Original" is always first and is whatever the file already does -- direct
// play, or a remux. The rungs below it exist only when something can encode
// them, and never above the source. The unsupported-to-transcode rewrite of
// the original rung used to live here; the planner (internal/playback) owns
// that decision now, so base.Mode arrives already final.
func qualityLadder(media library.FileMedia, base stream.Decision, canTranscode bool) []videoQuality {
	ladder := []videoQuality{{Height: 0, Mode: string(base.Mode)}}

	if !canTranscode || media.Status != library.MediaOK || media.Video == nil {
		return ladder
	}
	for _, height := range stream.AvailableHeights(media.Video.Height) {
		ladder = append(ladder, videoQuality{Height: height, Mode: string(stream.ModeTranscode)})
	}
	return ladder
}
