package ffmpeg

import (
	"context"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// What this machine can decode with, on the way into a transcode.
//
// Companion to capabilities.go and built on the same principle: the pinned
// ffmpeg build lists every hwaccel on every platform because they are all
// compiled in, and whether one runs depends on the card and the driver. So each
// candidate is asked to decode one frame, and the ones that come back are the
// ones considered.
//
// The measured caveat, which is why the order below is not alphabetical. A probe
// proves a method *starts*, not that it is *faster*. On the maintainer's AMD
// desktop, transcoding one HEVC Main 10 rip to 720p:
//
//	none (software decode)  8.1x real time
//	d3d11va                 9.86x
//	dxva2                   5.37x
//	auto                    3.8x
//
// Two of those are slower than no acceleration at all. `auto` is therefore not a
// candidate: letting ffmpeg choose picked the worst of them. dxva2 sits behind
// d3d11va for the same reason, and is kept only as the fallback for Windows
// installs too old for d3d11va.
//
// If the chosen method ever turns out slower in real use, the fix is to replace
// this liveness probe with a short benchmark against software decoding. That is
// more honest and more expensive; it is not worth paying until it is needed.

// Decoder is one usable hardware decoding path.
type Decoder struct {
	// Name is the ffmpeg -hwaccel value, e.g. "d3d11va".
	Name string `json:"name"`

	// Platforms it is offered on. Empty means every platform.
	platforms []string

	// priority orders the candidates. Lower is preferred.
	priority int
}

// decoderCandidates are tried in this order, best first.
var decoderCandidates = []Decoder{
	{Name: "d3d11va", platforms: []string{"windows"}, priority: 10},
	{Name: "videotoolbox", platforms: []string{"darwin"}, priority: 20},
	{Name: "cuda", platforms: []string{"linux", "windows"}, priority: 30},
	{Name: "vaapi", platforms: []string{"linux"}, priority: 40},
	{Name: "dxva2", platforms: []string{"windows"}, priority: 50},
}

func (d Decoder) availableHere() bool {
	if len(d.platforms) == 0 {
		return true
	}
	for _, platform := range d.platforms {
		if platform == runtime.GOOS {
			return true
		}
	}
	return false
}

// HardwareDecoder is the -hwaccel a transcode should use, if any.
//
// The empty string is a real answer, not a failure: software decoding already
// runs at eight times real time on the machine this was measured on, and it is
// correct everywhere.
//
// Probed once, lazily, exactly like the encoders: a library that never
// transcodes must not pay for it.
func (m *Manager) HardwareDecoder(ctx context.Context) string {
	m.decoderOnce.Do(func() {
		m.decoder = m.probeDecoders(ctx)
	})
	return m.decoder
}

func (m *Manager) probeDecoders(ctx context.Context) string {
	binary, err := m.Path(ctx)
	if err != nil {
		m.log.Warn("no ffmpeg, so no hardware decoding", "error", err)
		return ""
	}

	// A driver that hangs on a probe must not hang the playback that asked for
	// it. Shorter than the encoder ceiling: there are fewer candidates and a
	// decoder that does not answer quickly is not one to route a film through.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var (
		mu      sync.Mutex
		usable  []Decoder
		refused []string
		wait    sync.WaitGroup
	)
	for _, candidate := range decoderCandidates {
		if !candidate.availableHere() {
			continue
		}
		wait.Add(1)
		go func(candidate Decoder) {
			defer wait.Done()
			if err := probeDecoder(ctx, binary, candidate.Name); err != nil {
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
	wait.Wait()

	sort.Slice(usable, func(i, j int) bool { return usable[i].priority < usable[j].priority })
	sort.Strings(refused)

	names := make([]string, 0, len(usable))
	for _, decoder := range usable {
		names = append(names, decoder.Name)
	}
	chosen := ""
	if len(usable) > 0 {
		chosen = usable[0].Name
	}
	m.log.Info("hardware decoders probed",
		"usable", strings.Join(names, ","),
		"refused", strings.Join(refused, ","),
		"chosen", chosen)

	return chosen
}

// probeDecoder asks the accelerator to open and hand back one frame.
//
// The input is generated rather than read from the library: this must answer
// "does this driver initialise" without depending on a file being present, and
// without reading somebody's film to find out.
func probeDecoder(ctx context.Context, binary, name string) error {
	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error",
		"-hwaccel", name,
		"-f", "lavfi", "-i", "testsrc=s=640x360:d=0.1",
		"-frames:v", "1",
		"-f", "null", "-",
	)
	return cmd.Run()
}
