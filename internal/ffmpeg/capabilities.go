package ffmpeg

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// What this machine can actually encode with.
//
// The distinction that makes this file necessary: the pinned ffmpeg build lists
// h264_nvenc, h264_qsv, h264_amf, h264_mf and libx264 on every platform,
// because they are all compiled in. Whether any of them runs depends on the
// card in the machine and the driver on top of it. Measured on the maintainer's
// own desktop, `-encoders` offers all five and only three start: NVENC cannot
// load nvcuda.dll and QSV cannot create an MFX session, both instantly, because
// there is no NVIDIA card and no Intel graphics.
//
// So nothing here is inferred from a list. Each candidate is asked to encode one
// frame of nothing, and the ones that come back are the ones offered. That costs
// a few hundred milliseconds, once, on the first playback that needs a
// transcode -- and it is the only answer that is not a guess.

// EncoderKind separates what a viewer needs to know from what the log needs.
type EncoderKind string

const (
	// KindHardware runs on a GPU or a fixed-function block.
	KindHardware EncoderKind = "hardware"

	// KindSoftware runs on the CPU. Correct everywhere, and the one that costs
	// something a household notices.
	KindSoftware EncoderKind = "software"
)

// Encoder is one usable way of producing H.264.
type Encoder struct {
	// Name is the ffmpeg encoder, e.g. "h264_amf".
	Name string `json:"name"`

	Kind EncoderKind `json:"kind"`

	// Vendor is for the log and for a support question, never for a sentence
	// shown to somebody: "amd", "nvidia", "intel", "windows", "cpu".
	Vendor string `json:"vendor"`

	// priority orders the candidates. Lower is preferred.
	priority int
}

// candidates are tried in this order.
//
// Vendor-specific encoders come before MediaFoundation because they expose the
// controls this pipeline needs -- a bitrate that is honoured, a usable keyframe
// interval -- while h264_mf is a thin shim over whatever Windows decides.
// libx264 is last and always present: it is the floor, not a choice.
var candidates = []Encoder{
	{Name: "h264_nvenc", Kind: KindHardware, Vendor: "nvidia", priority: 10},
	{Name: "h264_qsv", Kind: KindHardware, Vendor: "intel", priority: 20},
	{Name: "h264_amf", Kind: KindHardware, Vendor: "amd", priority: 30},
	{Name: "h264_videotoolbox", Kind: KindHardware, Vendor: "apple", priority: 40},
	{Name: "h264_vaapi", Kind: KindHardware, Vendor: "vaapi", priority: 50},
	{Name: "h264_mf", Kind: KindHardware, Vendor: "windows", priority: 60},
	{Name: "libx264", Kind: KindSoftware, Vendor: "cpu", priority: 100},
}

// Capabilities is the answer, computed once per process.
type Capabilities struct {
	Encoders []Encoder `json:"encoders"`
}

// Best is the encoder a transcode should use: hardware if any of it works.
func (c Capabilities) Best() (Encoder, bool) {
	if len(c.Encoders) == 0 {
		return Encoder{}, false
	}
	return c.Encoders[0], true
}

// Kind reports what the best available encoder runs on, for the interface to
// say what a quality change will cost.
func (c Capabilities) Kind() EncoderKind {
	if best, ok := c.Best(); ok {
		return best.Kind
	}
	return KindSoftware
}

// Capabilities probes once and remembers.
//
// Deliberately lazy and never called at startup: a library of browser-friendly
// files never transcodes anything, and it must not pay for five subprocesses to
// learn about a furnace it will never light. The first quality change pays, and
// nothing after it does.
func (m *Manager) Capabilities(ctx context.Context) Capabilities {
	m.capsOnce.Do(func() {
		m.caps = m.probeEncoders(ctx)
	})
	return m.caps
}

func (m *Manager) probeEncoders(ctx context.Context) Capabilities {
	binary, err := m.Path(ctx)
	if err != nil {
		m.log.Warn("no ffmpeg, so no encoder is available", "error", err)
		return Capabilities{}
	}

	// A short ceiling on the whole exercise. A driver that hangs on a probe
	// must not hang the playback that asked for it.
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	var (
		mu       sync.Mutex
		usable   []Encoder
		refused  []string
		waitting sync.WaitGroup
	)
	for _, candidate := range candidates {
		waitting.Add(1)
		go func(candidate Encoder) {
			defer waitting.Done()
			if err := probeEncoder(ctx, binary, candidate.Name); err != nil {
				mu.Lock()
				refused = append(refused, candidate.Name)
				mu.Unlock()
				return
			}
			mu.Lock()
			usable = append(usable, candidate)
			mu.Unlock()
		}(candidate)
	}
	waitting.Wait()

	sort.Slice(usable, func(i, j int) bool { return usable[i].priority < usable[j].priority })
	sort.Strings(refused)

	names := make([]string, 0, len(usable))
	for _, encoder := range usable {
		names = append(names, encoder.Name)
	}
	m.log.Info("encoders probed",
		"usable", strings.Join(names, ","),
		"refused", strings.Join(refused, ","))

	return Capabilities{Encoders: usable}
}

// probeEncoder asks for one frame of nothing. Success means the encoder opened,
// which is the only claim that matters; failure means a missing card, a missing
// driver, or a build without it.
func probeEncoder(ctx context.Context, binary, name string) error {
	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "nullsrc=s=640x360:d=0.1",
		"-frames:v", "1",
		"-c:v", name,
		"-f", "null", "-",
	)
	return cmd.Run()
}
