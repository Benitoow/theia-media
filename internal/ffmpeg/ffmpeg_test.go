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
	if want := 12*time.Second + 30*time.Millisecond; info.Duration != want {
		t.Errorf("duration = %v, want %v", info.Duration, want)
	}
	if info.Container != "matroska,webm" {
		t.Errorf("container = %q, want matroska,webm", info.Container)
	}
	if info.Video.StreamIndex != 0 || info.Video.Width != 640 || info.Video.Height != 360 {
		t.Errorf("video stream = %+v, want stream 0 at 640x360", info.Video)
	}
	if len(info.AudioStreams) != 1 || info.AudioStreams[0].StreamIndex != 1 ||
		!info.AudioStreams[0].Default || info.AudioStreams[0].Channels != "mono" {
		t.Errorf("audio streams = %+v, want default mono stream 1", info.AudioStreams)
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
    Metadata:
      title           : Français 5.1
  Stream #0:3(fre): Subtitle: subrip`

	info := parseProbeOutput(output)
	if info.VideoCodec != "hevc" {
		t.Errorf("video = %q, want hevc", info.VideoCodec)
	}
	if info.AudioCodec != "truehd" {
		t.Errorf("audio = %q, want the first track (truehd)", info.AudioCodec)
	}
	if info.Video.Width != 3840 || info.Video.Height != 2160 {
		t.Errorf("video dimensions = %dx%d, want 3840x2160", info.Video.Width, info.Video.Height)
	}
	if len(info.AudioStreams) != 2 {
		t.Fatalf("audio streams = %+v, want two", info.AudioStreams)
	}
	if first := info.AudioStreams[0]; first.StreamIndex != 1 || first.Language != "eng" || first.Channels != "7.1" {
		t.Errorf("first audio stream = %+v", first)
	}
	if second := info.AudioStreams[1]; second.StreamIndex != 2 || second.Language != "fre" ||
		second.Channels != "5.1(side)" || second.Title != "Français 5.1" {
		t.Errorf("second audio stream = %+v", second)
	}
	if want := 2*time.Hour + 35*time.Minute + 12*time.Second + 450*time.Millisecond; info.Duration != want {
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

func TestCanonicalContainerUsesTheFileExtensionToResolveFFmpegAliases(t *testing.T) {
	tests := []struct {
		format, extension, want string
	}{
		{"mov,mp4,m4a,3gp,3g2,mj2", ".mp4", "mp4"},
		{"mov,mp4,m4a,3gp,3g2,mj2", ".m4v", "mp4"},
		{"matroska,webm", ".mkv", "matroska"},
		{"matroska,webm", ".webm", "webm"},
		{"mpegts", ".ts", "ts"},
	}
	for _, tt := range tests {
		if got := canonicalContainer(tt.format, tt.extension); got != tt.want {
			t.Errorf("canonicalContainer(%q, %q) = %q, want %q",
				tt.format, tt.extension, got, tt.want)
		}
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

// The frame rate, taken verbatim from the maintainer's own 4K file.
//
// The line is copied unchanged out of a real probe rather than written by hand,
// because what makes this parse hard is the company the number keeps: tbr and
// tbn print lookalike values on the same line, and one of them is 1k.
func TestParseProbeOutputReadsTheFrameRate(t *testing.T) {
	const output = `Input #0, matroska,webm, from 'Dune.mkv':
  Duration: 02:35:26.61, start: 0.000000, bitrate: 11924 kb/s
  Stream #0:0: Video: hevc (Main 10), yuv420p10le(tv, bt2020nc/bt2020/smpte2084), 3840x1604, SAR 1:1 DAR 960:401, 23.98 fps, 23.98 tbr, 1k tbn (default)
  Stream #0:1(fre): Audio: truehd (Dolby TrueHD + Dolby Atmos), 48000 Hz, 7.1, s32 (24 bit) (default)`

	info := parseProbeOutput(output)
	if info.Video.FrameRate != 23.98 {
		t.Errorf("frame rate = %v, want 23.98", info.Video.FrameRate)
	}
	if info.Video.Width != 3840 || info.Video.Height != 1604 {
		t.Errorf("dimensions = %dx%d, want 3840x1604", info.Video.Width, info.Video.Height)
	}
}

// A whole number, and a stream that never says. Both are ordinary.
func TestParseProbeOutputFrameRateEdges(t *testing.T) {
	whole := parseProbeOutput(
		"  Stream #0:0: Video: h264, yuv420p, 1920x1080, 24 fps, 24 tbr, 90k tbn\n")
	if whole.Video.FrameRate != 24 {
		t.Errorf("whole frame rate = %v, want 24", whole.Video.FrameRate)
	}

	// Zero is the honest answer, and the caller has to treat it as "unknown"
	// rather than as a slow film.
	silent := parseProbeOutput("  Stream #0:0: Video: mpeg2video, yuv420p, 720x576\n")
	if silent.Video.FrameRate != 0 {
		t.Errorf("frame rate = %v, want 0 when the line does not say", silent.Video.FrameRate)
	}
}
