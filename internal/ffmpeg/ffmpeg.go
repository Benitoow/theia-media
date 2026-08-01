// Package ffmpeg obtains and runs the one external dependency Theia has.
//
// The binary is downloaded on first *need* rather than at first launch: a
// library of browser-friendly files never triggers it, and someone who only
// ever direct-plays never spends 80 MB. It lands in the application's data
// directory and is never embedded in the Theia binary.
//
// Nothing is executed before its SHA-256 has been checked against the constant
// pinned below. The download is a binary fetched over the network and then run
// as a subprocess; verifying it is not optional.
package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrUnsupportedPlatform means no pinned build exists for this OS and
	// architecture. Direct play still works; only remuxing is unavailable.
	ErrUnsupportedPlatform = errors.New("ffmpeg: no build is pinned for this platform")

	// ErrMediaUnreadable distinguishes a file ffmpeg could not identify as
	// video from a failure to download or execute ffmpeg itself.
	ErrMediaUnreadable = errors.New("ffmpeg: media is unreadable")
)

// releaseTag is an immutable git tag, which matters more than it looks.
//
// The obvious alternatives publish under moving tags -- BtbN/FFmpeg-Builds uses
// "latest", which is republished in place -- and a pinned checksum against a
// moving target fails the day upstream rebuilds. eugeneware/ffmpeg-static tags
// each build, covers every OS and architecture Theia ships for, and hosts on
// GitHub Releases, which is one of the two hosts this project is allowed to
// contact at all.
const releaseTag = "b6.1.1"

const downloadBase = "https://github.com/eugeneware/ffmpeg-static/releases/download/" + releaseTag + "/"

// build is one pinned artifact. The digests come from the GitHub release API,
// which reports a sha256 for every asset, so they were pinned without having to
// trust a separately published checksum file.
type build struct {
	asset  string
	sha256 string
}

var builds = map[string]build{
	"windows/amd64": {"ffmpeg-win32-x64", "04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00"},
	// No native Windows ARM64 build exists upstream. Windows runs x64 binaries
	// under emulation, so the x64 one is used rather than leaving the platform
	// without remuxing entirely.
	"windows/arm64": {"ffmpeg-win32-x64", "04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00"},
	"linux/amd64":   {"ffmpeg-linux-x64", "e7e7fb30477f717e6f55f9180a70386c62677ef8a4d4d1a5d948f4098aa3eb99"},
	"linux/arm64":   {"ffmpeg-linux-arm64", "6bb182d0d75d23028db82e9e4f723ca69b853d055698486e6984ddb2c06fb8ce"},
	"darwin/amd64":  {"ffmpeg-darwin-x64", "ebdddc936f61e14049a2d4b549a412b8a40deeff6540e58a9f2a2da9e6b18894"},
	"darwin/arm64":  {"ffmpeg-darwin-arm64", "a90e3db6a3fd35f6074b013f948b1aa45b31c6375489d39e572bea3f18336584"},
}

// Manager owns the local copy of ffmpeg.
type Manager struct {
	dir  string
	log  *slog.Logger
	http *http.Client

	// One download at a time. Two viewers starting a remux at once must not
	// both fetch 80 MB over each other.
	mu       sync.Mutex
	verified bool
}

// New prepares a manager. dir is where the binary is kept.
func New(dir string, log *slog.Logger) *Manager {
	return &Manager{
		dir:  dir,
		log:  log,
		http: &http.Client{Timeout: 15 * time.Minute},
	}
}

// binaryName is what the file is called on disk.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

// Supported reports whether a build is pinned for this platform.
func Supported() bool {
	_, ok := builds[runtime.GOOS+"/"+runtime.GOARCH]
	return ok
}

// Available reports whether ffmpeg is already on disk. It does not download and
// does not hash -- it answers "would playing this need a download first".
func (m *Manager) Available() bool {
	info, err := os.Stat(filepath.Join(m.dir, binaryName()))
	return err == nil && !info.IsDir() && info.Size() > 0
}

// Path returns the ffmpeg binary, downloading and verifying it if needed.
//
// The first call may take a while; every later one is a stat. Callers should
// treat the first invocation as user-visible work.
func (m *Manager) Path(ctx context.Context) (string, error) {
	spec, ok := builds[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", ErrUnsupportedPlatform
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	target := filepath.Join(m.dir, binaryName())

	if m.verified {
		return target, nil
	}

	// A file already there is re-hashed once per process. Cheap insurance
	// against a half-finished download from a previous run, or a swapped
	// binary.
	if _, err := os.Stat(target); err == nil {
		switch err := verify(target, spec.sha256); {
		case err == nil:
			m.verified = true
			return target, nil
		default:
			m.log.Warn("the cached ffmpeg failed verification and will be downloaded again",
				"error", err)
			os.Remove(target)
		}
	}

	if err := m.download(ctx, spec, target); err != nil {
		return "", err
	}
	m.verified = true
	return target, nil
}

func (m *Manager) download(ctx context.Context, spec build, target string) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("ffmpeg: creating %s: %w", m.dir, err)
	}

	url := downloadBase + spec.asset
	m.log.Info("downloading ffmpeg", "url", url, "destination", m.dir)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("ffmpeg: building the download request: %w", err)
	}
	res, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("ffmpeg: downloading: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ffmpeg: downloading: unexpected status %d", res.StatusCode)
	}

	// Downloaded to a temporary name with no executable bit. It is only given
	// one, and only moved into place, after the digest matches -- an unverified
	// binary must never be runnable, however briefly.
	tmp, err := os.CreateTemp(m.dir, ".ffmpeg-download-*")
	if err != nil {
		return fmt.Errorf("ffmpeg: creating a temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, digest), res.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("ffmpeg: downloading: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("ffmpeg: writing the download: %w", err)
	}

	got := hex.EncodeToString(digest.Sum(nil))
	if got != spec.sha256 {
		return fmt.Errorf("ffmpeg: checksum mismatch for %s: expected %s, got %s",
			spec.asset, spec.sha256, got)
	}

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fmt.Errorf("ffmpeg: making the binary executable: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("ffmpeg: installing the binary: %w", err)
	}

	m.log.Info("ffmpeg installed", "path", target, "sha256", spec.sha256[:12]+"…")
	return nil
}

// verify hashes a file on disk against an expected digest.
func verify(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", want, got)
	}
	return nil
}

// MediaInfo is what ffmpeg measured about a file. VideoCodec and AudioCodec
// remain as compatibility shortcuts for the first streams; V2-M1 consumers use
// Video and AudioStreams so a human can choose a real audio track.
type MediaInfo struct {
	Container    string        `json:"container,omitempty"`
	VideoCodec   string        `json:"video_codec"`
	AudioCodec   string        `json:"audio_codec"`
	Video        VideoStream   `json:"video"`
	AudioStreams []AudioStream `json:"audio_streams"`
	Duration     time.Duration `json:"-"`
	Seconds      float64       `json:"duration_seconds"`
}

type VideoStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type AudioStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Language    string `json:"language,omitempty"`
	Title       string `json:"title,omitempty"`
	Channels    string `json:"channels,omitempty"`
	Default     bool   `json:"default"`
}

var (
	// "  Stream #0:1(eng): Audio: ac3, 48000 Hz, 5.1(side), fltp, 448 kb/s"
	streamPattern     = regexp.MustCompile(`^\s*Stream #\d+:(\d+)(?:\[[^\]]*\])?(?:\(([^)]*)\))?: (Video|Audio|Subtitle|Data|Attachment): ([A-Za-z0-9_]+)(.*)$`)
	inputPattern      = regexp.MustCompile(`^Input #\d+,\s*(.+?),\s+from\s`)
	resolutionPattern = regexp.MustCompile(`(?:^|,\s)(\d{2,5})x(\d{2,5})(?:\s|\[|,|$)`)
	channelsPattern   = regexp.MustCompile(`\d+\s*Hz,\s*(mono|stereo|\d+(?:\.\d+)?(?:\([^)]*\))?)`)
	metadataPattern   = regexp.MustCompile(`^\s*(title|language)\s*:\s*(.*?)\s*$`)
	// "  Duration: 00:01:30.05, start: 0.000000, bitrate: 1234 kb/s"
	durationPattern = regexp.MustCompile(`Duration: (\d+):(\d\d):(\d\d)\.(\d+)`)
)

// Probe reports the codecs in a file.
//
// This shells out to ffmpeg rather than ffprobe because the pinned upstream
// ships only the one binary. Running ffmpeg with an input and no output prints
// the stream table and exits non-zero on purpose -- the exit code is therefore
// ignored, and only the absence of a video stream is treated as failure.
func (m *Manager) Probe(ctx context.Context, path string) (MediaInfo, error) {
	binary, err := m.Path(ctx)
	if err != nil {
		return MediaInfo{}, err
	}

	cmd := exec.CommandContext(ctx, binary, "-hide_banner", "-i", path)
	output, runErr := cmd.CombinedOutput()
	if err := ctx.Err(); err != nil {
		return MediaInfo{}, err
	}
	// A normal probe exits non-zero because no output file was requested. A
	// failure to start the verified executable is different: callers must not
	// blame the media file or cache a false inspection error.
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return MediaInfo{}, fmt.Errorf("ffmpeg: starting probe: %w", runErr)
	}

	info := parseProbeOutput(string(output))
	if info.VideoCodec == "" {
		return info, fmt.Errorf("%w: no video stream found in %s", ErrMediaUnreadable, filepath.Base(path))
	}
	info.Container = canonicalContainer(info.Container, filepath.Ext(path))
	return info, nil
}

// canonicalContainer turns ffmpeg's demuxer aliases into the name a person can
// use to distinguish files. For example, MP4 probes as
// "mov,mp4,m4a,3gp,3g2,mj2" and MKV as "matroska,webm"; returning that whole
// implementation list would be accurate and useless.
func canonicalContainer(format, extension string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	extension = strings.TrimPrefix(strings.ToLower(extension), ".")
	aliases := map[string][]string{
		"mp4":  {"mov", "mp4"},
		"m4v":  {"mov", "mp4"},
		"mov":  {"mov"},
		"mkv":  {"matroska"},
		"webm": {"webm"},
		"ts":   {"mpegts"},
		"m2ts": {"mpegts"},
		"mts":  {"mpegts"},
		"avi":  {"avi"},
	}
	for _, candidate := range aliases[extension] {
		for _, reported := range strings.Split(format, ",") {
			if strings.TrimSpace(reported) == candidate {
				switch extension {
				case "m4v":
					return "mp4"
				case "mkv":
					return "matroska"
				default:
					return extension
				}
			}
		}
	}
	if before, _, found := strings.Cut(format, ","); found {
		return strings.TrimSpace(before)
	}
	return format
}

// parseProbeOutput reads the stream table ffmpeg prints. Split out from Probe
// so the parsing -- regexes over free text, the fragile half -- can be tested
// without spawning anything.
func parseProbeOutput(output string) MediaInfo {
	info := MediaInfo{AudioStreams: []AudioStream{}}
	currentAudio := -1

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if info.Container == "" {
			if match := inputPattern.FindStringSubmatch(line); match != nil {
				info.Container = strings.ToLower(strings.TrimSpace(match[1]))
			}
		}

		if match := streamPattern.FindStringSubmatch(line); match != nil {
			streamIndex, _ := strconv.Atoi(match[1])
			language := strings.ToLower(strings.TrimSpace(match[2]))
			kind := match[3]
			codec := strings.ToLower(match[4])
			rest := match[5]
			currentAudio = -1

			switch kind {
			case "Video":
				if info.VideoCodec == "" {
					info.VideoCodec = codec
					info.Video = VideoStream{StreamIndex: streamIndex, Codec: codec}
					if size := resolutionPattern.FindStringSubmatch(rest); size != nil {
						info.Video.Width, _ = strconv.Atoi(size[1])
						info.Video.Height, _ = strconv.Atoi(size[2])
					}
				}
			case "Audio":
				track := AudioStream{
					StreamIndex: streamIndex,
					Codec:       codec,
					Language:    language,
					Default:     strings.Contains(strings.ToLower(rest), "(default)"),
				}
				if channels := channelsPattern.FindStringSubmatch(rest); channels != nil {
					track.Channels = strings.ToLower(channels[1])
				}
				info.AudioStreams = append(info.AudioStreams, track)
				currentAudio = len(info.AudioStreams) - 1
				if info.AudioCodec == "" {
					info.AudioCodec = codec
				}
			}
			continue
		}

		// ffmpeg prints per-stream title/language on following Metadata lines.
		// currentAudio is reset by every next stream, so subtitle metadata cannot
		// leak into the preceding audio track.
		if currentAudio >= 0 {
			if match := metadataPattern.FindStringSubmatch(line); match != nil {
				switch strings.ToLower(match[1]) {
				case "title":
					info.AudioStreams[currentAudio].Title = strings.TrimSpace(match[2])
				case "language":
					if info.AudioStreams[currentAudio].Language == "" {
						info.AudioStreams[currentAudio].Language = strings.ToLower(strings.TrimSpace(match[2]))
					}
				}
			}
		}
	}

	if d := durationPattern.FindStringSubmatch(output); d != nil {
		h, _ := strconv.Atoi(d[1])
		mn, _ := strconv.Atoi(d[2])
		s, _ := strconv.Atoi(d[3])
		fraction, _ := strconv.ParseFloat("0."+d[4], 64)
		info.Seconds = float64(h*3600+mn*60+s) + fraction
		info.Duration = time.Duration(info.Seconds * float64(time.Second))
	}
	return info
}
