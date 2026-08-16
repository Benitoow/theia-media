package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
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
// candidate: letting ffmpeg choose picked the worst of them.
//
// That note ended by saying that if the chosen method ever turned out slower in
// real use, the liveness probe should be replaced by a short benchmark against
// software decoding -- more honest, more expensive, and not worth paying for
// until it was needed. It is needed. Measured on a Ryzen AI 9 HX 370 with the
// integrated Radeon 890M, the same 720p transcode:
//
//	none (software decode)  5.85x real time
//	d3d11va                 4.07x
//
// The same flag that won by 22% on a desktop with a card of its own loses by 30%
// on a laptop whose GPU shares memory with the CPU, because `-hwaccel` without a
// GPU filter chain decodes on the GPU and then copies every frame back for the
// scaler. Whether that copy is worth making is a property of the machine, and no
// amount of ordering a candidate list can answer it. So the list below now says
// only which methods are worth *trying*, and benchmarkDecoders decides.
//
// Software decoding remains the floor and the default: it ran at 5.85x and 8.1x
// on the two machines measured, is correct everywhere, and costs nothing to be
// wrong about.

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

	// A driver that hangs must not hang the playback that asked for it. The
	// benchmark below runs a handful of two-second decodes one after another,
	// so this is wider than the old liveness probe's fifteen seconds -- and it
	// is still a ceiling, not a budget: exceeding it means software decoding,
	// which is the correct answer anyway.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	clip, cleanup, err := benchmarkClip(ctx, binary)
	if err != nil {
		// No sample means nothing to time. Software decoding is correct
		// everywhere, so this is a slower answer rather than a broken one.
		m.log.Warn("could not build a decoder benchmark clip, decoding in software",
			"error", err)
		return ""
	}
	defer cleanup()

	// Software decoding is the baseline every candidate has to beat, and it is
	// also the fallback if none of them does.
	base, err := timeDecode(ctx, binary, "", clip)
	if err != nil {
		m.log.Warn("could not time software decoding, decoding in software", "error", err)
		return ""
	}

	var (
		usable  []Decoder
		refused []string
		timings = map[string]time.Duration{"none": base}
	)
	// Sequential, unlike the old probe. Two accelerators timed at once contend
	// for the same silicon and both come out looking slow, which is exactly the
	// measurement error this replaced a probe to avoid.
	for _, candidate := range decoderCandidates {
		if !candidate.availableHere() {
			continue
		}
		took, err := timeDecode(ctx, binary, candidate.Name, clip)
		if err != nil {
			refused = append(refused, candidate.Name)
			continue
		}
		timings[candidate.Name] = took
		usable = append(usable, candidate)
	}
	chosen := chooseDecoder(base, usable, timings)

	sort.Strings(refused)
	names := make([]string, 0, len(usable))
	for _, decoder := range usable {
		names = append(names, decoder.Name+"="+timings[decoder.Name].Round(time.Millisecond).String())
	}
	m.log.Info("hardware decoders benchmarked",
		"software", base.Round(time.Millisecond).String(),
		"timed", strings.Join(names, ","),
		"refused", strings.Join(refused, ","),
		"chosen", chosen)

	return chosen
}

// benchmarkClip writes a short compressed sample to decode.
//
// It has to be a real encoded stream: the old liveness probe fed the
// accelerator a lavfi pattern, which is generated as raw frames, so the decoder
// under test had nothing to decode and any hwaccel "passed". Two seconds of
// 1080p H.264 is enough to separate the paths and quick enough to build.
//
// Generated rather than taken from the library, for the same reason as before:
// finding out what this machine can do must not depend on a film being present,
// and must not read somebody's film to answer.
func benchmarkClip(ctx context.Context, binary string) (path string, cleanup func(), err error) {
	file, err := os.CreateTemp("", "theia-decoder-*.mp4")
	if err != nil {
		return "", nil, err
	}
	path = file.Name()
	file.Close()
	cleanup = func() { os.Remove(path) }

	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=24",
		"-t", "2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		path,
	)
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

// timeDecode measures one decoding path in the shape a transcode uses it.
//
// The scale filter is not incidental. `-hwaccel` on its own decodes on the GPU
// and then copies every frame back to system memory for a CPU filter, and that
// copy is the whole reason a hardware path can come out slower than none. Timing
// a bare decode would hide precisely the cost being looked for.
//
// name is the -hwaccel value, or the empty string for software decoding.
func timeDecode(ctx context.Context, binary, name, clip string) (time.Duration, error) {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if name != "" {
		args = append(args, "-hwaccel", name)
	}
	args = append(args,
		"-i", clip,
		"-vf", "scale=-2:720",
		"-f", "null", "-",
	)

	start := time.Now()
	if err := exec.CommandContext(ctx, binary, args...).Run(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// mustBeat is how much faster than software decoding a hardware path has to be
// before it is worth taking. A candidate inside the noise keeps software
// decoding, which is the one path correct on every machine and the one it costs
// nothing to be wrong about.
const mustBeat = 0.9

// chooseDecoder picks the fastest path worth taking, or "" for software.
//
// Separated from the timing so the rule can be tested without running ffmpeg:
// the arithmetic here is what decides whether a household's films transcode at
// four times real time or six, and it should not need a GPU to check.
func chooseDecoder(software time.Duration, usable []Decoder, timings map[string]time.Duration) string {
	sort.Slice(usable, func(i, j int) bool { return usable[i].priority < usable[j].priority })

	chosen := ""
	best := software
	for _, candidate := range usable {
		took, ok := timings[candidate.Name]
		if !ok || took <= 0 {
			continue
		}
		if took < time.Duration(float64(best)*mustBeat) {
			best = took
			chosen = candidate.Name
		}
	}
	return chosen
}
