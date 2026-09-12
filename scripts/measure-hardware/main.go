// Command measure-hardware runs the tranche-6 measurement campaign of
// docs/plan-refonte-lecture.md on one machine, and prints the numbers the
// report publishes.
//
// It measures rather than infers, on the pattern the runtime probes already
// follow (decoders.go): every candidate is asked to do the work, and only what
// comes back, timed, is reported. Nothing here changes a delivery decision --
// the codec target stays H.264 SDR and the browser keeps its escalation; the
// point is to know, per machine that runs the script, which measured chain a
// 4K HEVC source should actually take, and whether today's hybrid path is it.
//
// Usage:
//
//	go run ./scripts/measure-hardware -ffmpeg <pinned ffmpeg> [-out report.md]
//
// The script builds synthetic sources (testsrc2, encoded with libx265 and
// libx264), times each chain twice and keeps the faster run, and compares the
// outputs of the fastest hardware chain against the full-software chain with
// ffmpeg's own PSNR filter on rasterized frames.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// encoderCandidates is the matrix probed per codec. Vendor-specific encoders
// are listed before the platform shims, libx264/libx265 last: the runtime's
// Best() keeps its own order, and this list only says what to try.
var encoderCandidates = map[string][]struct {
	name      string
	platforms []string
}{
	"h264": {
		{"h264_nvenc", nil},
		{"h264_qsv", nil},
		{"h264_amf", []string{"windows"}},
		{"h264_videotoolbox", []string{"darwin"}},
		{"h264_vaapi", []string{"linux"}},
		{"h264_mf", []string{"windows"}},
		{"libx264", nil},
	},
	"hevc": {
		{"hevc_nvenc", nil},
		{"hevc_qsv", nil},
		{"hevc_amf", []string{"windows"}},
		{"hevc_videotoolbox", []string{"darwin"}},
		{"hevc_vaapi", []string{"linux"}},
		{"hevc_mf", []string{"windows"}},
		{"libx265", nil},
	},
	"av1": {
		{"av1_nvenc", nil},
		{"av1_qsv", nil},
		{"av1_amf", []string{"windows"}},
		{"libsvtav1", nil},
	},
}

// decodeCandidates is the -hwaccel matrix, platform-gated like the runtime's.
var decodeCandidates = []struct {
	name      string
	platforms []string
}{
	{"d3d11va", []string{"windows"}},
	{"dxva2", []string{"windows"}},
	{"videotoolbox", []string{"darwin"}},
	{"cuda", []string{"linux", "windows"}},
	{"vaapi", []string{"linux"}},
}

func availableHere(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, platform := range platforms {
		if platform == runtime.GOOS {
			return true
		}
	}
	return false
}

func main() {
	ffmpegFlag := flag.String("ffmpeg", os.Getenv("THEIA_TEST_FFMPEG"), "path to the pinned ffmpeg binary")
	outFlag := flag.String("out", "", "also write the report to this file")
	flag.Parse()
	if *ffmpegFlag == "" {
		fmt.Fprintln(os.Stderr, "measure-hardware: -ffmpeg is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	report := &strings.Builder{}
	printf := func(format string, args ...any) {
		fmt.Fprintf(report, format, args...)
		fmt.Printf(format, args...)
	}

	printf("# Tranche-6 hardware measurements\n\n")
	printf("Machine: %s/%s, %d logical CPUs. FFmpeg: %s\n\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), *ffmpegFlag)

	// 1. Liveness per encoder, one frame each.
	printf("## Encoders probed (one frame, refused means the driver said no)\n\n")
	printf("| Codec | Encoder | Verdict |\n|---|---|---|\n")
	for _, codec := range []string{"h264", "hevc", "av1"} {
		for _, candidate := range encoderCandidates[codec] {
			if !availableHere(candidate.platforms) {
				continue
			}
			verdict := "usable"
			if err := probeEncoder(ctx, *ffmpegFlag, candidate.name); err != nil {
				verdict = "refused"
			}
			printf("| %s | `%s` | %s |\n", codec, candidate.name, verdict)
		}
	}
	printf("\n")

	workdir, err := os.MkdirTemp("", "theia-measure-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "measure-hardware:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(workdir)

	// 2. Sources. The 4K HEVC one is the acceptance scenario; the 1080p H.264
	// one is the everyday case.
	printf("## Sources\n\n")
	source4k, err := buildSource(ctx, *ffmpegFlag, filepath.Join(workdir, "source-4k-hevc.mp4"),
		"testsrc2=size=3840x2160:rate=24", "libx265", []string{"-preset", "ultrafast", "-pix_fmt", "yuv420p"})
	if err != nil {
		printf("the 4K HEVC source could not be built (%v); the 4K scenarios are skipped.\n\n", err)
	}
	source1080, err := buildSource(ctx, *ffmpegFlag, filepath.Join(workdir, "source-1080-h264.mp4"),
		"testsrc2=size=1920x1080:rate=24", "libx264", []string{"-preset", "ultrafast", "-pix_fmt", "yuv420p"})
	if err != nil {
		printf("the 1080p H.264 source could not be built (%v).\n\n", err)
	} else {
		printf("- 1080p H.264 SDR, 10 s: %s\n", humanSize(source1080))
	}
	if source4k != "" {
		printf("- 4K HEVC SDR, 10 s: %s\n\n", humanSize(source4k))
	}

	decodes := []string{""}
	for _, candidate := range decodeCandidates {
		if availableHere(candidate.platforms) {
			decodes = append(decodes, candidate.name)
		}
	}

	// 3. The decoder question on the 4K source: does the machine-level choice
	// the runtime benchmark made on a 1080p H.264 clip hold for 4K HEVC?
	if source4k != "" {
		printf("## Decode of the 4K HEVC source, to 720p, no encoder (10 s source)\n\n")
		printf("| Decode | Wall time | Real time |\n|---|---:|---:|\n")
		for _, decode := range decodes {
			took, ok := bestOf(2, func() (time.Duration, error) {
				return timeDecode(ctx, *ffmpegFlag, decode, source4k)
			})
			if !ok {
				printf("| `%s` | refused | - |\n", decodeName(decode))
				continue
			}
			printf("| `%s` | %s | %.2fx |\n", decodeName(decode),
				took.Round(time.Millisecond), 10/took.Seconds())
		}
		printf("\n")
	}

	// 4. The chains: 4K HEVC to 720p H.264, every live encoder against every
	// decode path. This is the acceptance scenario.
	if source4k != "" {
		printf("## Chains: 4K HEVC → 720p H.264 (10 s source, faster of two runs)\n\n")
		printf("| Encode | Decode | Wall time | Real time |\n|---|---|---:|---:|\n")
		type chain struct {
			encoder string
			decode  string
			took    time.Duration
		}
		var chains []chain
		encoders := liveEncoders(ctx, *ffmpegFlag, "h264")
		for _, encoder := range encoders {
			for _, decode := range decodes {
				took, ok := bestOf(2, func() (time.Duration, error) {
					return timeChain(ctx, *ffmpegFlag, encoder, decode, source4k, 720, 2160, os.DevNull)
				})
				if !ok {
					printf("| `%s` | `%s` | refused | - |\n", encoder, decodeName(decode))
					continue
				}
				printf("| `%s` | `%s` | %s | %.2fx |\n", encoder, decodeName(decode),
					took.Round(time.Millisecond), 10/took.Seconds())
				chains = append(chains, chain{encoder: encoder, decode: decode, took: took})
			}
		}
		sort.Slice(chains, func(i, j int) bool { return chains[i].took < chains[j].took })
		if len(chains) > 1 {
			// 5. Quality: the fastest chain against the full-software one, on
			// rasterized frames. Both are 720p H.264, so PSNR is a direct
			// comparison; anything below ~35 dB would be visible and would
			// refuse the chain.
			fastest, software := chains[0], chain{}
			for _, candidate := range chains {
				if candidate.encoder == "libx264" && candidate.decode == "" {
					software = candidate
					break
				}
			}
			if software.encoder != "" {
				fastestOut := filepath.Join(workdir, "fastest.mp4")
				softwareOut := filepath.Join(workdir, "software.mp4")
				_, err1 := timeChain(ctx, *ffmpegFlag, fastest.encoder, fastest.decode, source4k, 720, 2160, fastestOut)
				_, err2 := timeChain(ctx, *ffmpegFlag, software.encoder, software.decode, source4k, 720, 2160, softwareOut)
				if err1 == nil && err2 == nil {
					psnr, psnrErr := measurePSNR(ctx, *ffmpegFlag, fastestOut, softwareOut)
					if psnrErr != nil {
						printf("\nQuality comparison could not run: %v\n", psnrErr)
					} else {
						printf("\nQuality: fastest chain (`%s`+`%s`) against full software, PSNR %.2f dB\n",
							fastest.encoder, decodeName(fastest.decode), psnr)
					}
				}
			}
		}
		printf("\n")
	}

	// 6. The everyday case, for reference.
	if source1080 != "" {
		printf("## Chains: 1080p H.264 → 720p H.264 (10 s source, faster of two runs)\n\n")
		printf("| Encode | Decode | Wall time | Real time |\n|---|---|---:|---:|\n")
		for _, encoder := range liveEncoders(ctx, *ffmpegFlag, "h264") {
			for _, decode := range decodes {
				took, ok := bestOf(2, func() (time.Duration, error) {
					return timeChain(ctx, *ffmpegFlag, encoder, decode, source1080, 720, 1080, os.DevNull)
				})
				if !ok {
					printf("| `%s` | `%s` | refused | - |\n", encoder, decodeName(decode))
					continue
				}
				printf("| `%s` | `%s` | %s | %.2fx |\n", encoder, decodeName(decode),
					took.Round(time.Millisecond), 10/took.Seconds())
			}
		}
		printf("\n")
	}

	// 7. HEVC encoding speed, for the record. Delivery targets stay H.264;
	// this is the number the next conversation about HEVC targets will want.
	if source4k != "" {
		printf("## HEVC encode of the 4K source → 720p (for the record; not a delivery target)\n\n")
		printf("| Encode | Decode | Wall time | Real time |\n|---|---|---:|---:|\n")
		for _, encoder := range liveEncoders(ctx, *ffmpegFlag, "hevc") {
			took, ok := bestOf(2, func() (time.Duration, error) {
				return timeChain(ctx, *ffmpegFlag, encoder, "", source4k, 720, 2160, os.DevNull)
			})
			if !ok {
				printf("| `%s` | none | refused | - |\n", encoder)
				continue
			}
			printf("| `%s` | none | %s | %.2fx |\n", encoder, took.Round(time.Millisecond), 10/took.Seconds())
		}
		printf("\n")
	}

	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(report.String()), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "measure-hardware: writing the report:", err)
			os.Exit(1)
		}
	}
}

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

func liveEncoders(ctx context.Context, binary, codec string) []string {
	var live []string
	for _, candidate := range encoderCandidates[codec] {
		if !availableHere(candidate.platforms) {
			continue
		}
		if err := probeEncoder(ctx, binary, candidate.name); err == nil {
			live = append(live, candidate.name)
		}
	}
	return live
}

func buildSource(ctx context.Context, binary, path, source, encoder string, extra []string) (string, error) {
	args := []string{"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", source, "-t", "10",
		"-c:v", encoder}
	args = append(args, extra...)
	args = append(args, path)
	cmd := exec.CommandContext(ctx, binary, args...)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return path, nil
}

// timeDecode answers the decode-only question in the shape timeDecode in
// internal/ffmpeg asks it: a bare decode with a CPU scale, which is where the
// copy-back cost a hardware path pays shows up.
func timeDecode(ctx context.Context, binary, hwaccel, source string) (time.Duration, error) {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if hwaccel != "" {
		args = append(args, "-hwaccel", hwaccel)
	}
	args = append(args, "-i", source, "-vf", "scale=-2:720", "-f", "null", "-")
	start := time.Now()
	if err := exec.CommandContext(ctx, binary, args...).Run(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// timeChain runs one encode+decode chain the way the delivery would: the same
// shape of arguments the runtime's TranscodeArgs builds, output to a file or
// to the void.
func timeChain(ctx context.Context, binary, encoder, hwaccel, source string, height, sourceHeight int, output string) (time.Duration, error) {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if hwaccel != "" {
		args = append(args, "-hwaccel", hwaccel)
	}
	args = append(args,
		"-i", source,
		"-map", "0:v:0",
		"-c:v", encoder,
		"-vf", fmt.Sprintf("scale=-2:%d,setsar=1", height),
		"-b:v", "8M", "-maxrate", "8M", "-bufsize", "12M",
		"-g", "48",
		"-pix_fmt", "yuv420p",
		"-an",
	)
	if encoder == "libx264" {
		args = append(args, "-preset", "veryfast")
	}
	// The fragmented-MP4 flags are not decoration: the mp4 muxer refuses a
	// non-seekable pipe without empty_moov, which is why the delivery writes
	// them and this harness must too.
	args = append(args, "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-f", "mp4")
	if output == os.DevNull {
		args = append(args, "pipe:1")
	} else {
		args = append(args, "-y", output)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if output == os.DevNull {
		devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return 0, err
		}
		defer devnull.Close()
		cmd.Stdout = devnull
	}
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%w: %s", err, oneLine(stderr.String(), 400))
	}
	return time.Since(start), nil
}

func oneLine(value string, maximum int) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	if len(value) > maximum {
		return value[len(value)-maximum:]
	}
	return value
}

func measurePSNR(ctx context.Context, binary, candidate, reference string) (float64, error) {
	out, err := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "info",
		"-i", candidate, "-i", reference,
		"-lavfi", "psnr", "-f", "null", "-",
	).CombinedOutput()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if index := strings.Index(line, "average:"); index >= 0 {
			var value float64
			if _, err := fmt.Sscanf(line[index:], "average:%f", &value); err == nil {
				return value, nil
			}
		}
	}
	return 0, fmt.Errorf("no psnr line in ffmpeg output")
}

// bestOf runs the measurement and keeps the fastest of the runs that came
// back: two runs of a GPU chain differ by more than the differences being
// measured, and the faster run is the honest lower bound.
func bestOf(runs int, measure func() (time.Duration, error)) (time.Duration, bool) {
	best, have := time.Duration(0), false
	for i := 0; i < runs; i++ {
		took, err := measure()
		if err != nil {
			continue
		}
		if !have || took < best {
			best, have = took, true
		}
	}
	return best, have
}

func decodeName(decode string) string {
	if decode == "" {
		return "none"
	}
	return decode
}

func humanSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown size"
	}
	return fmt.Sprintf("%.1f MiB", float64(info.Size())/(1<<20))
}
