package ffmpeg

import (
	"testing"
	"time"
)

// The rule this file pins was paid for twice, on two machines that disagreed.
// On an AMD desktop with a card of its own, d3d11va transcoded a 720p stream at
// 9.86x against software's 8.1x. On a laptop whose Radeon 890M shares memory
// with the CPU, the same flag ran at 4.07x against software's 5.85x -- because
// `-hwaccel` without a GPU filter chain copies every decoded frame back for the
// scaler, and whether that copy is worth making is a property of the machine.
//
// So the ordering of decoderCandidates no longer decides anything. These are the
// cases that do.

func windows(name string, priority int) Decoder {
	return Decoder{Name: name, priority: priority}
}

func TestAFasterHardwarePathIsTaken(t *testing.T) {
	// The maintainer's desktop, in the proportions actually measured.
	usable := []Decoder{windows("d3d11va", 10), windows("dxva2", 50)}
	timings := map[string]time.Duration{
		"d3d11va": 812 * time.Millisecond,
		"dxva2":   1490 * time.Millisecond,
	}

	if got := chooseDecoder(time.Second, usable, timings); got != "d3d11va" {
		t.Errorf("chooseDecoder = %q, want d3d11va: it was measurably the fastest", got)
	}
}

func TestASlowerHardwarePathIsRefused(t *testing.T) {
	// The laptop. This is the case the old liveness probe got wrong: d3d11va
	// initialised, so it was chosen, and every transcode on that machine ran a
	// third slower than doing nothing special at all.
	usable := []Decoder{windows("d3d11va", 10), windows("dxva2", 50)}
	timings := map[string]time.Duration{
		"d3d11va": 368 * time.Millisecond,
		"dxva2":   436 * time.Millisecond,
	}

	if got := chooseDecoder(174*time.Millisecond, usable, timings); got != "" {
		t.Errorf("chooseDecoder = %q, want software decoding: both paths were slower", got)
	}
}

func TestATieKeepsSoftwareDecoding(t *testing.T) {
	// Within the noise is not a win. Software decoding is correct on every
	// machine and costs nothing to be wrong about, so it keeps the tie.
	usable := []Decoder{windows("d3d11va", 10)}
	for _, close := range []time.Duration{1000, 960, 901} {
		timings := map[string]time.Duration{"d3d11va": close * time.Millisecond}
		if got := chooseDecoder(1000*time.Millisecond, usable, timings); got != "" {
			t.Errorf("chooseDecoder with hardware at %v = %q, want software", close, got)
		}
	}
	// Clearly ahead, however, is taken.
	timings := map[string]time.Duration{"d3d11va": 899 * time.Millisecond}
	if got := chooseDecoder(1000*time.Millisecond, usable, timings); got != "d3d11va" {
		t.Errorf("chooseDecoder = %q, want d3d11va once it is past the margin", got)
	}
}

func TestACandidateThatWasNeverTimedIsNotChosen(t *testing.T) {
	// A decoder that refused to run has no entry. It must not be picked on the
	// strength of a zero value comparing favourably with everything.
	usable := []Decoder{windows("cuda", 30)}
	if got := chooseDecoder(time.Second, usable, map[string]time.Duration{}); got != "" {
		t.Errorf("chooseDecoder = %q, want software: cuda was never timed", got)
	}
	if got := chooseDecoder(time.Second, usable, map[string]time.Duration{"cuda": 0}); got != "" {
		t.Errorf("chooseDecoder = %q, want software: a zero timing is not a fast one", got)
	}
}

func TestNoHardwareAtAllIsAnAnswer(t *testing.T) {
	if got := chooseDecoder(time.Second, nil, nil); got != "" {
		t.Errorf("chooseDecoder = %q, want the empty string", got)
	}
}
