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

	"github.com/Benitoow/theia-media/internal/boundedio"
)

var (
	// ErrUnsupportedPlatform means no pinned build exists for this OS and
	// architecture. Direct play still works; only remuxing is unavailable.
	ErrUnsupportedPlatform = errors.New("ffmpeg: no build is pinned for this platform")

	// ErrMediaUnreadable distinguishes a file ffmpeg could not identify as
	// video from a failure to download or execute ffmpeg itself.
	ErrMediaUnreadable = errors.New("ffmpeg: media is unreadable")
)

// releaseTag is an immutable GitHub release. Jellyfin's portable FFmpeg build
// is the newest stable source found with native artifacts for every platform
// Theia ships, including Windows ARM64. FFmpeg's own 9.x release currently has
// no single trustworthy binary source covering that six-platform matrix; using
// one runtime family everywhere is preferable to changing features by OS.
const releaseTag = "v8.1.2-4"

const expectedVersion = "ffmpeg version 8.1.2-Jellyfin"

const (
	previousRuntimeSuffix  = ".previous"
	unmanagedRuntimeSuffix = ".unmanaged"
	invalidRuntimeSuffix   = ".invalid"
)

const downloadBase = "https://github.com/jellyfin/jellyfin-ffmpeg/releases/download/" + releaseTag + "/"

// No pinned package is larger than 65 MiB. Keep enough room for packaging
// changes, but never let a broken or hostile response fill the data disk before
// its (necessarily wrong) digest can be rejected.
const maxPackageBytes int64 = 256 << 20

// build is one pinned package and the one executable Theia extracts from it.
// packageSHA256 is GitHub's digest for the release asset; binarySHA256 was
// measured after extracting that verified package. Both are required: the first
// protects the download, the second keeps every subsequent startup verifiable.
type build struct {
	asset         string
	packageSHA256 string
	binarySHA256  string
	format        archiveFormat
}

type legacyRuntime struct {
	binarySHA256    string
	expectedVersion string
}

// RuntimeManifest is the complete provenance of the external executable used
// on one platform. Keeping it queryable makes runtime upgrades reviewable
// without duplicating asset names and digests in diagnostics or scripts.
type RuntimeManifest struct {
	Release        string `json:"release"`
	Asset          string `json:"asset"`
	SHA256         string `json:"sha256"`
	DownloadSHA256 string `json:"download_sha256"`
	Source         string `json:"source"`
}

func Manifest() (RuntimeManifest, bool) {
	spec, ok := builds[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return RuntimeManifest{}, false
	}
	return RuntimeManifest{
		Release:        releaseTag,
		Asset:          spec.asset,
		SHA256:         spec.binarySHA256,
		DownloadSHA256: spec.packageSHA256,
		Source:         downloadBase + spec.asset,
	}, true
}

var builds = map[string]build{
	"windows/amd64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_win64-clang-gpl.zip",
		"a6821d72985ee6d5a8af16925b468d1c4ec1f652b582a0a2a5039282c26ffca5",
		"546580347aa7553ea9ac5fbcba05b787e423f9ce0685a71e7500b554cc21bc6c", archiveZip,
	},
	"windows/arm64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_winarm64-clang-gpl.zip",
		"f77e3d0b2dcb4bb7eec51e7240cceaee9071f715ab7d7efb4b6e263b04f75f0d",
		"ce0f3f8225fcd23c75a8786ba91d99dc6df8dcce696194878c967fca9befaac9", archiveZip,
	},
	"linux/amd64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_linux64-gpl.tar.xz",
		"6e7150c358f9817a04ce82c62d81135cb4535d8525047393f4b296fff3d7a664",
		"9302bc18b99a39de49bcb44be392b534340c2d27ded75f0c1225bae4612c24c9", archiveTarXZ,
	},
	"linux/arm64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_linuxarm64-gpl.tar.xz",
		"ceb9642ee513491d0440bc0027dfa33f2fc6c9966cabb0ec69f43edcbb853e84",
		"ebc8f52d82f71fa4d982a8a1954805e9605c72c7b8c4d20b84b74e63237f1d33", archiveTarXZ,
	},
	"darwin/amd64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_mac64-gpl.tar.xz",
		"d50d288cd321f12f506d91ef3c21cfdec34f160191a556d4da8fe73b3d1b22bd",
		"a8c942a96825aab9ca112dbc1b194109e38aae4f7adfb28e6f577666f9c6f913", archiveTarXZ,
	},
	"darwin/arm64": {
		"jellyfin-ffmpeg_8.1.2-4_portable_macarm64-gpl.tar.xz",
		"1362e5cd8399bb9d648f237b94fff86f864aa9b61a3f239cfcccd77f91bf2649",
		"e4947e53444dda2bc1fb1f6c0461b4691b93483ab76829f5e9a9c3f2dfa0b67b", archiveTarXZ,
	},
}

// legacyRuntimes contains only binaries that Theia itself previously shipped.
// They are allowed as a one-process fallback when the pinned replacement cannot
// be downloaded. An arbitrary executable dropped into the managed directory is
// preserved for the owner, but is never executed by Theia.
var legacyRuntimes = map[string][]legacyRuntime{
	"windows/amd64": {{"04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00", "ffmpeg version 6.1.1"}},
	"windows/arm64": {{"04e1307997530f9cf2fe35cba2ca7e8875ca91da02f89d6c7243df819c94ad00", "ffmpeg version 6.1.1"}},
	"linux/amd64":   {{"e7e7fb30477f717e6f55f9180a70386c62677ef8a4d4d1a5d948f4098aa3eb99", "ffmpeg version 6.1.1"}},
	"linux/arm64":   {{"6bb182d0d75d23028db82e9e4f723ca69b853d055698486e6984ddb2c06fb8ce", "ffmpeg version 6.1.1"}},
	"darwin/amd64":  {{"ebdddc936f61e14049a2d4b549a412b8a40deeff6540e58a9f2a2da9e6b18894", "ffmpeg version 6.1.1"}},
	"darwin/arm64":  {{"a90e3db6a3fd35f6074b013f948b1aa45b31c6375489d39e572bea3f18336584", "ffmpeg version 6.1.1"}},
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

	// What this machine can encode with. Probed once, on the first playback
	// that needs it -- see capabilities.go.
	capsOnce sync.Once
	caps     Capabilities

	// And what it can decode with on the way in. Same laziness, same reason --
	// see decoders.go. The empty string is a valid answer.
	decoderOnce sync.Once
	decoder     string

	probeMu sync.Mutex
	probes  map[string]*probeFlight
}

type probeFlight struct {
	done chan struct{}
	info MediaInfo
	err  error
}

// New prepares a manager. dir is where the binary is kept.
func New(dir string, log *slog.Logger) *Manager {
	return &Manager{
		dir:    dir,
		log:    log,
		http:   &http.Client{Timeout: 15 * time.Minute},
		probes: map[string]*probeFlight{},
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
	platform := runtime.GOOS + "/" + runtime.GOARCH
	spec, ok := builds[platform]
	if !ok {
		return "", ErrUnsupportedPlatform
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	target := filepath.Join(m.dir, binaryName())

	if m.verified {
		return target, nil
	}

	// The directory is owned by Theia. A current runtime is used, a runtime
	// shipped by an older Theia may carry one failed download, and every other
	// executable is quarantined only after its verified replacement is ready.
	// Nothing is deleted merely because GitHub or the network is unavailable.
	var (
		fallback     *legacyRuntime
		backupSuffix string
	)
	if _, err := os.Stat(target); err == nil {
		if err := verify(target, spec.binarySHA256); err == nil {
			if err := validateRuntime(ctx, target); err == nil {
				m.removeVerifiedLegacyBackup(target)
				m.verified = true
				return target, nil
			} else {
				m.log.Warn("the cached ffmpeg has the pinned hash but cannot be executed; a verified replacement will be prepared", "error", err)
				backupSuffix = invalidRuntimeSuffix
			}
		} else if legacy := matchingLegacyRuntime(target, legacyRuntimes[platform]); legacy != nil {
			fallback = legacy
			backupSuffix = previousRuntimeSuffix
			m.log.Info("a previous Theia ffmpeg runtime will be upgraded", "from", legacy.expectedVersion, "to", expectedVersion)
		} else {
			backupSuffix = unmanagedRuntimeSuffix
			m.log.Warn("the ffmpeg executable in Theia's managed directory was not installed by this or the previous release; it will not be executed")
		}
	}

	if err := m.downloadAndInstall(ctx, spec, target, backupSuffix); err != nil {
		if fallback != nil {
			if fallbackErr := validateRuntimeVersion(ctx, target, fallback.expectedVersion); fallbackErr == nil {
				m.log.Warn("the pinned ffmpeg update failed; this process will keep the verified previous Theia runtime", "error", err)
				m.verified = true
				return target, nil
			}
		}
		return "", err
	}
	m.verified = true
	return target, nil
}

func validateRuntime(ctx context.Context, path string) error {
	return validateRuntimeVersion(ctx, path, expectedVersion)
}

func validateRuntimeVersion(ctx context.Context, path, expected string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(checkCtx, path, "-version").Output()
	if err != nil {
		return fmt.Errorf("ffmpeg: validating runtime version: %w", err)
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	if !strings.Contains(first, expected) {
		return fmt.Errorf("ffmpeg: runtime reports %q, expected %q", first, expected)
	}
	return nil
}

func (m *Manager) download(ctx context.Context, spec build, target string) error {
	return m.downloadAndInstall(ctx, spec, target, "")
}

func (m *Manager) downloadAndInstall(ctx context.Context, spec build, target, backupSuffix string) error {
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
	if res.ContentLength > maxPackageBytes {
		return fmt.Errorf("ffmpeg: package is too large: %d bytes", res.ContentLength)
	}

	// The release asset and the executable extracted from it are two separate
	// trust boundaries. Both temporary files are non-executable; neither byte is
	// run until the GitHub package digest and the pinned runtime digest match.
	archiveFile, err := os.CreateTemp(m.dir, ".ffmpeg-download-*")
	if err != nil {
		return fmt.Errorf("ffmpeg: creating a temporary package: %w", err)
	}
	archiveName := archiveFile.Name()
	defer os.Remove(archiveName)

	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(archiveFile, digest), io.LimitReader(res.Body, maxPackageBytes+1))
	if err != nil {
		archiveFile.Close()
		return fmt.Errorf("ffmpeg: downloading: %w", err)
	}
	if written > maxPackageBytes {
		archiveFile.Close()
		return fmt.Errorf("ffmpeg: package exceeds %d bytes", maxPackageBytes)
	}
	if err := archiveFile.Close(); err != nil {
		return fmt.Errorf("ffmpeg: writing the package: %w", err)
	}

	got := hex.EncodeToString(digest.Sum(nil))
	if got != spec.packageSHA256 {
		return fmt.Errorf("ffmpeg: checksum mismatch for %s: expected %s, got %s",
			spec.asset, spec.packageSHA256, got)
	}

	runtimeFile, err := os.CreateTemp(m.dir, ".ffmpeg-runtime-*")
	if err != nil {
		return fmt.Errorf("ffmpeg: creating a temporary runtime: %w", err)
	}
	runtimeName := runtimeFile.Name()
	defer os.Remove(runtimeName)
	if err := runtimeFile.Close(); err != nil {
		return fmt.Errorf("ffmpeg: preparing the temporary runtime: %w", err)
	}

	if err := extractRuntime(archiveName, runtimeName, binaryName(), spec.format); err != nil {
		return fmt.Errorf("ffmpeg: extracting %s: %w", spec.asset, err)
	}
	if err := verify(runtimeName, spec.binarySHA256); err != nil {
		return fmt.Errorf("ffmpeg: extracted runtime verification failed: %w", err)
	}
	if err := os.Chmod(runtimeName, 0o755); err != nil {
		return fmt.Errorf("ffmpeg: making the binary executable: %w", err)
	}
	// Execute the staged file before touching what currently works. A correct
	// pair of hashes proves provenance; this proves the binary runs here.
	if err := validateRuntime(ctx, runtimeName); err != nil {
		return fmt.Errorf("ffmpeg: validating the staged runtime: %w", err)
	}
	if err := installRuntime(runtimeName, target, backupSuffix); err != nil {
		return fmt.Errorf("ffmpeg: installing the binary: %w", err)
	}

	m.log.Info("ffmpeg installed", "path", target, "sha256", spec.binarySHA256[:12]+"…")
	return nil
}

func matchingLegacyRuntime(path string, candidates []legacyRuntime) *legacyRuntime {
	for index := range candidates {
		if verify(path, candidates[index].binarySHA256) == nil {
			return &candidates[index]
		}
	}
	return nil
}

// installRuntime swaps only after the replacement has been fully verified and
// executed. Windows permits renaming a running executable but not replacing it
// in place, so the same two-renames protocol as Theia's updater is used here.
func installRuntime(staged, target, backupSuffix string) error {
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return os.Rename(staged, target)
	} else if err != nil {
		return err
	}
	if backupSuffix == "" {
		return fmt.Errorf("refusing to replace an existing runtime without a preservation class")
	}

	backup, err := availableRuntimeBackup(target + backupSuffix)
	if err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("install failed: %v; restoring the previous runtime failed: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func availableRuntimeBackup(base string) (string, error) {
	for index := 0; index < 100; index++ {
		candidate := base
		if index > 0 {
			candidate = fmt.Sprintf("%s.%d", base, index)
		}
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("too many preserved runtimes beside %s", base)
}

func (m *Manager) removeVerifiedLegacyBackup(target string) {
	for _, candidate := range legacyRuntimes[runtime.GOOS+"/"+runtime.GOARCH] {
		path := target + previousRuntimeSuffix
		if verify(path, candidate.binarySHA256) == nil {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				m.log.Debug("the previous ffmpeg runtime could not be removed", "path", path, "error", err)
			}
			return
		}
	}
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
	Container       string           `json:"container,omitempty"`
	VideoCodec      string           `json:"video_codec"`
	AudioCodec      string           `json:"audio_codec"`
	Video           VideoStream      `json:"video"`
	AudioStreams    []AudioStream    `json:"audio_streams"`
	SubtitleStreams []SubtitleStream `json:"subtitle_streams"`
	Duration        time.Duration    `json:"-"`
	Seconds         float64          `json:"duration_seconds"`
}

type VideoStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`

	// FrameRate is what the file runs at, as ffmpeg reports it on the stream
	// line. It exists so that "is the browser keeping up" can be asked as a
	// ratio against the truth rather than against a constant: decision 59 chose
	// a fixed floor of ten precisely because this was not stored, and a fixed
	// floor cannot tell a 4K decoder running at half speed from a healthy one.
	FrameRate float64 `json:"frame_rate,omitempty"`

	// ColorTransfer is the transfer function, and it is the whole question for
	// tone mapping: "smpte2084" is PQ and "arib-std-b67" is HLG, both of which
	// need converting before an SDR H.264 encode, and anything else -- including
	// nothing at all -- does not. Stored raw rather than as a boolean so the
	// interface can tell HDR10 from HLG without a second measurement.
	ColorTransfer string `json:"color_transfer,omitempty"`

	// DolbyVision is the presence of a DOVI configuration record beside the
	// stream. It changes no decision Theia makes -- the base layer is what gets
	// decoded either way -- and exists so the file can say what it is.
	DolbyVision bool `json:"dolby_vision,omitempty"`
}

type AudioStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Language    string `json:"language,omitempty"`
	Title       string `json:"title,omitempty"`
	Channels    string `json:"channels,omitempty"`
	Default     bool   `json:"default"`

	// Profile is what ffmpeg prints in brackets after the codec: "Dolby TrueHD +
	// Dolby Atmos", "DTS-HD MA". It is a name, not a decision -- nothing in the
	// playback path reads it -- and it is here so a file can be labelled with
	// what it actually holds.
	Profile string `json:"profile,omitempty"`
}

// SubtitleStream is one embedded subtitle track. The codec decides whether it
// can be rendered at all -- see package subtitles and decision 3 -- so it is
// reported even for the bitmap formats Theia refuses to burn in.
type SubtitleStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Language    string `json:"language,omitempty"`
	Title       string `json:"title,omitempty"`
	Default     bool   `json:"default"`
	Forced      bool   `json:"forced"`
}

var (
	// "  Stream #0:1(eng): Audio: ac3, 48000 Hz, 5.1(side), fltp, 448 kb/s"
	streamPattern     = regexp.MustCompile(`^\s*Stream #\d+:(\d+)(?:\[[^\]]*\])?(?:\(([^)]*)\))?: (Video|Audio|Subtitle|Data|Attachment): ([A-Za-z0-9_]+)(.*)$`)
	inputPattern      = regexp.MustCompile(`^Input #\d+,\s*(.+?),\s+from\s`)
	resolutionPattern = regexp.MustCompile(`(?:^|,\s)(\d{2,5})x(\d{2,5})(?:\s|\[|,|$)`)
	// "…, 3840x1604, SAR 1:1 DAR 960:401, 23.98 fps, 23.98 tbr, 1k tbn". Anchored
	// on the unit, because tbr and tbn carry lookalike numbers on the same line.
	frameRatePattern = regexp.MustCompile(`(?:^|,\s)(\d+(?:\.\d+)?)\s+fps(?:\s|,|$)`)
	// The transfer function, anchored on the two names that change what has to
	// happen to the picture. The colour group ffmpeg prints beside the pixel
	// format varies -- "yuv420p10le(tv, bt2020nc/bt2020/smpte2084)", or a single
	// token, or nothing -- so matching the names is more robust than parsing the
	// shape, and no other field on the line carries either word.
	hdrTransferPattern = regexp.MustCompile(`\b(smpte2084|arib-std-b67)\b`)
	// "  Stream #0:1(fre): Audio: truehd (Dolby TrueHD + Dolby Atmos), 48000 Hz"
	// -- the bracket immediately after the codec, and only that one.
	audioProfilePattern = regexp.MustCompile(`^\s*\(([^)]+)\)`)
	channelsPattern     = regexp.MustCompile(`\d+\s*Hz,\s*(mono|stereo|\d+(?:\.\d+)?(?:\([^)]*\))?)`)
	metadataPattern     = regexp.MustCompile(`^\s*(title|language)\s*:\s*(.*?)\s*$`)
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
	key := path
	if stat, err := os.Stat(path); err == nil {
		key = fmt.Sprintf("%s\x00%d\x00%d", path, stat.Size(), stat.ModTime().UnixNano())
	}
	m.probeMu.Lock()
	if active := m.probes[key]; active != nil {
		m.probeMu.Unlock()
		select {
		case <-ctx.Done():
			return MediaInfo{}, ctx.Err()
		case <-active.done:
			return active.info, active.err
		}
	}
	flight := &probeFlight{done: make(chan struct{})}
	m.probes[key] = flight
	m.probeMu.Unlock()

	flight.info, flight.err = m.probe(ctx, path)
	m.probeMu.Lock()
	delete(m.probes, key)
	close(flight.done)
	m.probeMu.Unlock()
	return flight.info, flight.err
}

func (m *Manager) probe(ctx context.Context, path string) (MediaInfo, error) {
	binary, err := m.Path(ctx)
	if err != nil {
		return MediaInfo{}, err
	}

	cmd := exec.CommandContext(ctx, binary, "-hide_banner", "-i", path)
	output := boundedio.NewTail(1 << 20)
	cmd.Stdout, cmd.Stderr = output, output
	runErr := cmd.Run()
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

	info := parseProbeOutput(output.String())
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
	info := MediaInfo{AudioStreams: []AudioStream{}, SubtitleStreams: []SubtitleStream{}}
	// Which stream the Metadata lines that follow belong to. Both are reset by
	// every stream header, so a subtitle's title can never land on the audio
	// track above it.
	currentAudio, currentSubtitle := -1, -1

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
			currentAudio, currentSubtitle = -1, -1

			switch kind {
			case "Video":
				if info.VideoCodec == "" {
					info.VideoCodec = codec
					info.Video = VideoStream{StreamIndex: streamIndex, Codec: codec}
					if size := resolutionPattern.FindStringSubmatch(rest); size != nil {
						info.Video.Width, _ = strconv.Atoi(size[1])
						info.Video.Height, _ = strconv.Atoi(size[2])
					}
					// Not inside the resolution block: a stream can report a
					// rate without a size ffmpeg chose to print, and the two
					// are independent facts.
					if rate := frameRatePattern.FindStringSubmatch(rest); rate != nil {
						info.Video.FrameRate, _ = strconv.ParseFloat(rate[1], 64)
					}
					info.Video.ColorTransfer = strings.ToLower(
						hdrTransferPattern.FindString(rest))
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
				if profile := audioProfilePattern.FindStringSubmatch(rest); profile != nil {
					track.Profile = strings.TrimSpace(profile[1])
				}
				info.AudioStreams = append(info.AudioStreams, track)
				currentAudio = len(info.AudioStreams) - 1
				if info.AudioCodec == "" {
					info.AudioCodec = codec
				}
			case "Subtitle":
				lowerRest := strings.ToLower(rest)
				info.SubtitleStreams = append(info.SubtitleStreams, SubtitleStream{
					StreamIndex: streamIndex,
					Codec:       codec,
					Language:    language,
					Default:     strings.Contains(lowerRest, "(default)"),
					Forced:      strings.Contains(lowerRest, "(forced)"),
				})
				currentSubtitle = len(info.SubtitleStreams) - 1
			}
			continue
		}

		// The DOVI record sits under a "Side data:" heading below the video
		// stream and matches neither the stream nor the metadata pattern, so it
		// is read on its own. There is no cursor to keep: only the first video
		// stream is retained, and the record belongs to it.
		if info.Video.Codec != "" && strings.Contains(line, "DOVI configuration record") {
			info.Video.DolbyVision = true
			continue
		}

		// ffmpeg prints per-stream title/language on following Metadata lines.
		// Both cursors are reset by every next stream, so a subtitle's title
		// cannot leak into the audio track printed above it.
		if match := metadataPattern.FindStringSubmatch(line); match != nil {
			key := strings.ToLower(match[1])
			value := strings.TrimSpace(match[2])
			switch {
			case currentAudio >= 0 && key == "title":
				info.AudioStreams[currentAudio].Title = value
			case currentAudio >= 0 && key == "language":
				if info.AudioStreams[currentAudio].Language == "" {
					info.AudioStreams[currentAudio].Language = strings.ToLower(value)
				}
			case currentSubtitle >= 0 && key == "title":
				info.SubtitleStreams[currentSubtitle].Title = value
			case currentSubtitle >= 0 && key == "language":
				if info.SubtitleStreams[currentSubtitle].Language == "" {
					info.SubtitleStreams[currentSubtitle].Language = strings.ToLower(value)
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
