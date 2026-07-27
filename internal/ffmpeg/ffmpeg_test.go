package ffmpeg

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"runtime"
	"testing"
	"time"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// realOutput is what ffmpeg 6.1.1 actually printed for one of the M4 test
// clips. Kept verbatim rather than idealised, because the parser has to survive
// the real formatting, not a tidy version of it.
const realOutput = `Input #0, matroska,webm, from 'Remux H264 AC3.mkv':
  Metadata:
    ENCODER         : Lavf60.16.100
  Duration: 00:00:12.03, start: 0.000000, bitrate: 1990 kb/s
  Stream #0:0: Video: h264 (High 4:4:4 Predictive), yuv444p(progressive), 640x360 [SAR 1:1 DAR 16:9], 25 fps, 25 tbr, 1k tbn (default)
    Metadata:
      ENCODER         : Lavc60.31.102 libx264
  Stream #0:1: Audio: ac3, 48000 Hz, mono, fltp, 192 kb/s (default)
    Metadata:
      ENCODER         : Lavc60.31.102 ac3
At least one output file must be specified`

func TestParseProbeOutput(t *testing.T) {
	info := parseProbeOutput(realOutput)

	if info.VideoCodec != "h264" {
		t.Errorf("video = %q, want h264", info.VideoCodec)
	}
	if info.AudioCodec != "ac3" {
		t.Errorf("audio = %q, want ac3", info.AudioCodec)
	}
	if want := 12 * time.Second; info.Duration != want {
		t.Errorf("duration = %v, want %v", info.Duration, want)
	}
}

func TestParseProbeOutputHandlesLanguageTagsAndMultipleTracks(t *testing.T) {
	// A real film: tracks carry language tags, and there are usually several
	// audio streams. The first of each kind is what the remux maps, so it is
	// what the parser has to report.
	const output = `Input #0, matroska,webm, from 'Film.mkv':
  Duration: 02:35:12.45, start: 0.000000, bitrate: 24000 kb/s
  Stream #0:0(eng): Video: hevc (Main 10), yuv420p10le(tv), 3840x2160
  Stream #0:1(eng): Audio: truehd, 48000 Hz, 7.1, s32 (24 bit)
  Stream #0:2(fre): Audio: ac3, 48000 Hz, 5.1(side), fltp, 640 kb/s
  Stream #0:3(fre): Subtitle: subrip`

	info := parseProbeOutput(output)
	if info.VideoCodec != "hevc" {
		t.Errorf("video = %q, want hevc", info.VideoCodec)
	}
	if info.AudioCodec != "truehd" {
		t.Errorf("audio = %q, want the first track (truehd)", info.AudioCodec)
	}
	if want := 2*time.Hour + 35*time.Minute + 12*time.Second; info.Duration != want {
		t.Errorf("duration = %v, want %v", info.Duration, want)
	}
}

func TestParseProbeOutputOnSomethingThatIsNotVideo(t *testing.T) {
	// A sparse or truncated file. The caller turns an empty video codec into a
	// clear 415 rather than starting a doomed remux.
	const output = `[matroska @ 0000] EBML header parsing failed
Remux H264 AC3.mkv: Invalid data found when processing input`

	if info := parseProbeOutput(output); info.VideoCodec != "" {
		t.Errorf("video = %q, want empty for unreadable input", info.VideoCodec)
	}
}

func TestEveryShippedPlatformHasAPinnedBuild(t *testing.T) {
	// The six targets CI cross-compiles. A platform missing here would ship a
	// binary that can never remux anything.
	for _, platform := range []string{
		"windows/amd64", "windows/arm64",
		"linux/amd64", "linux/arm64",
		"darwin/amd64", "darwin/arm64",
	} {
		build, ok := builds[platform]
		if !ok {
			t.Errorf("no ffmpeg build is pinned for %s", platform)
			continue
		}
		if build.asset == "" {
			t.Errorf("%s has no asset name", platform)
		}
		// A digest that is not a full SHA-256 would silently never match, and
		// the failure would only appear on that platform.
		if len(build.sha256) != sha256.Size*2 {
			t.Errorf("%s: digest %q is not a 64-character SHA-256", platform, build.sha256)
			continue
		}
		if _, err := hex.DecodeString(build.sha256); err != nil {
			t.Errorf("%s: digest is not hexadecimal: %v", platform, err)
		}
	}
}

func TestSupportedReportsThisPlatform(t *testing.T) {
	if !Supported() {
		t.Errorf("Supported() = false on %s/%s, which is a platform Theia ships for",
			runtime.GOOS, runtime.GOARCH)
	}
}

func TestAvailableDoesNotDownload(t *testing.T) {
	// The promise this whole package is built around: asking whether ffmpeg is
	// present must never fetch 80 MB.
	m := New(t.TempDir(), discardLogger())
	if m.Available() {
		t.Error("Available() = true for an empty directory")
	}
}
