// Package preview builds the strip of small frames a player shows under the
// cursor while somebody drags the seek bar.
//
// It is a comfort, not a feature anything depends on, and the whole package is
// written to behave like one: it never blocks playback, never downloads
// anything, never fails a request, and answers "not yet" as a normal state.
package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/boundedio"
	"github.com/Benitoow/theia-media/internal/stream"
	"github.com/Benitoow/theia-media/internal/workload"
)

// ErrNotReady means the sheet does not exist yet. The caller should say so and
// let the player carry on without previews; a later request may find it built.
var ErrNotReady = errors.New("preview: not built yet")

// ErrUnavailable means no sheet will be built: no ffmpeg on disk, or nothing
// long enough to sample.
var ErrUnavailable = errors.New("preview: unavailable")

// Grid shape.
//
// tiles is the ceiling on how many frames one film is reduced to. A hundred
// frames over two hours is one every seventy seconds, which is enough to know
// which scene the cursor is over and is what the strip is for. More would be a
// larger download and a longer encode for a picture nobody studies.
//
// height is the tile height in pixels; the width follows the source aspect.
// Ninety is legible at the size a scrub preview is actually drawn and keeps a
// full sheet inside a couple of hundred kilobytes.
const (
	tiles       = 100
	columns     = 10
	tileHeight  = 90
	minInterval = 2.0

	// Below this there is nothing to scrub. A three-minute file is navigated by
	// dragging, not by looking.
	minDuration = 120.0
)

// The card preview: six seconds, half a thousand lines, no sound.
//
// It exists because the card is where a viewer decides, and a still is a poor
// answer to "what is this". It is deliberately small and short - it is drawn at
// the size of a card and it is fetched the moment a pointer rests on one - so
// the cost of a hundred cards is a hundred small files rather than a hundred
// encodes nobody watched. Built by the same manager as the strip, with the same
// slot, the same cache and the same promise never to get in playback's way.
const (
	clipSeconds  = 6
	clipHeight   = 480
	clipMinFloor = 60.0
	clipTailRoom = 90.0

	// Below this a clip is not worth building: the start would land in the
	// opening titles for anything short, and the sample would say nothing.
	minClipSource = 45.0
)

// ClipStart is where a clip begins in a file of this duration, in seconds.
//
// A fifth of the way in is past the titles for anything feature length; the
// floor keeps a short file out of its own title card, and the tail room keeps
// the sample from landing in the credits, where a preview shows a black frame
// and a scroll of names.
func ClipStart(duration float64) float64 {
	start := duration * 0.2
	if start < clipMinFloor {
		start = clipMinFloor
	}
	if latest := duration - clipTailRoom; start > latest {
		start = latest
	}
	if start < 0 {
		start = 0
	}
	return start
}

// Manifest is what the player needs to turn a cursor position into a tile.
type Manifest struct {
	// Key identifies this sheet, and is the only part of the path a client is
	// given. It is a digest of the file's identity, so a file replaced on disk
	// gets a different sheet rather than a stale one.
	Key string `json:"key"`

	IntervalSeconds float64 `json:"interval_seconds"`
	Columns         int     `json:"columns"`
	Rows            int     `json:"rows"`
	Count           int     `json:"count"`

	// TileHeight is fixed; the width is whatever the source's aspect made it.
	// The client divides the loaded image's natural width by Columns rather
	// than being told, because the pinned ffmpeg build ships no ffprobe and
	// measuring the sheet on this side would mean guessing an aspect ratio the
	// file may not have.
	TileHeight int `json:"tile_height"`
}

// keyPattern is what a client may ask for. Hex only: the key reaches the
// filesystem, and everything else about this package is best-effort, so the one
// place that must not be relaxed is written down as a rule rather than trusted.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Binary is the part of ffmpeg.Manager this package uses. An interface so that
// the package can be tested without a real ffmpeg, and so that "is it on disk"
// stays a question this package must ask rather than one it can skip.
type Binary interface {
	// Available reports whether ffmpeg is already downloaded. It must not
	// download.
	Available() bool

	// Path returns the binary. Only ever called once Available said yes, so it
	// resolves without fetching anything.
	Path(ctx context.Context) (string, error)
}

// Manager builds and serves sheets.
type Manager struct {
	dir    string
	ffmpeg Binary
	log    *slog.Logger

	// building holds the keys currently being generated, so twenty range
	// requests during one film do not start twenty encodes of it.
	//
	// failed holds the ones that could not be built. Without it a file ffmpeg
	// cannot read is re-attempted on every single request -- observed as three
	// identical failures in a third of a second while a player polled. It is
	// deliberately not persisted: a restart is usually an upgrade, and an
	// upgrade is a reason to try again.
	mu       sync.Mutex
	building map[string]bool
	failed   map[string]bool

	// slot admits one encode at a time. Decision 58 measured that a single
	// software transcode consumes the whole real-time margin on this machine;
	// a preview is worth far less than the film playing, so it queues rather
	// than competing.
	slot chan struct{}
	jobs *workload.Coordinator
}

// New prepares the cache directory.
func New(dir string, binary Binary, log *slog.Logger) (*Manager, error) {
	return NewWithCoordinator(dir, binary, log, nil)
}

// NewWithCoordinator gives playback authority to cancel and postpone preview
// generation. New remains for small embeddings and tests.
func NewWithCoordinator(dir string, binary Binary, log *slog.Logger, jobs *workload.Coordinator) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating the preview cache %s: %w", dir, err)
	}
	return &Manager{
		dir:      dir,
		ffmpeg:   binary,
		log:      log,
		building: map[string]bool{},
		failed:   map[string]bool{},
		slot:     make(chan struct{}, 1),
		jobs:     jobs,
	}, nil
}

// Key identifies a sheet from the file it describes.
//
// Size and modification time are in it as well as the path, so that replacing a
// film with a different encode of the same name produces a different sheet
// instead of showing frames from the file that is gone.
func Key(path string, size int64, modifiedUnix int64) string {
	sum := sha256.Sum256([]byte(path + "\x00" + strconv.FormatInt(size, 10) +
		"\x00" + strconv.FormatInt(modifiedUnix, 10)))
	return hex.EncodeToString(sum[:16])
}

// Lookup returns the manifest for a file, starting a build if there is not one.
//
// It never waits for the build. A player asks once when it opens a film and
// again if the user starts scrubbing; by then a short film is usually ready and
// a long one is not, which is exactly what ErrNotReady is for.
func (m *Manager) Lookup(ctx context.Context, key, source string, duration float64,
	colorTransfer string,
) (Manifest, error) {
	if !keyPattern.MatchString(key) {
		return Manifest{}, ErrUnavailable
	}
	if duration < minDuration {
		return Manifest{}, ErrUnavailable
	}

	manifest, err := m.read(key)
	if err == nil {
		return manifest, nil
	}

	// No sheet on disk. Building one needs ffmpeg, and asking for a preview
	// must never be the thing that downloads it -- the same promise /info makes.
	if m.ffmpeg == nil || !m.ffmpeg.Available() {
		return Manifest{}, ErrUnavailable
	}

	m.start("sheet:"+key, source, duration, func(ctx context.Context) error {
		return m.build(ctx, key, source, duration, colorTransfer)
	})
	return Manifest{}, ErrNotReady
}

// SheetPath returns the file to serve for a key, or ErrNotReady.
func (m *Manager) SheetPath(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", ErrUnavailable
	}
	path := filepath.Join(m.dir, key+".jpg")
	if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
		return "", ErrNotReady
	}
	return path, nil
}

// LookupClip is Lookup for the card preview: same three states, same promise
// that asking never blocks and never downloads anything.
func (m *Manager) LookupClip(ctx context.Context, key, source string, duration float64,
	colorTransfer string,
) error {
	if !keyPattern.MatchString(key) {
		return ErrUnavailable
	}
	if duration < minClipSource {
		return ErrUnavailable
	}
	if _, err := m.ClipPath(key); err == nil {
		return nil
	}
	if m.ffmpeg == nil || !m.ffmpeg.Available() {
		return ErrUnavailable
	}
	m.start("clip:"+key, source, duration, func(ctx context.Context) error {
		return m.buildClip(ctx, key, source, duration, colorTransfer)
	})
	return ErrNotReady
}

// ClipPath returns the file to serve for a key, or ErrNotReady.
func (m *Manager) ClipPath(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", ErrUnavailable
	}
	path := filepath.Join(m.dir, key+".mp4")
	if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
		return "", ErrNotReady
	}
	return path, nil
}

func (m *Manager) read(key string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(m.dir, key+".json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Count == 0 {
		return Manifest{}, fs.ErrNotExist
	}
	return manifest, nil
}

// start kicks off one build, unless that entry is already being built.
//
// The entry is named by the caller and names both the file and the thing being
// made - `sheet:<key>`, `clip:<key>` - so a strip that failed does not stop the
// card preview from being tried, which is what a single key would have done.
func (m *Manager) start(entry, source string, duration float64, work func(context.Context) error) {
	m.mu.Lock()
	if m.building[entry] || m.failed[entry] {
		m.mu.Unlock()
		return
	}
	m.building[entry] = true
	m.mu.Unlock()

	go func() {
		failed := true
		defer func() {
			m.mu.Lock()
			delete(m.building, entry)
			if failed {
				m.failed[entry] = true
			}
			m.mu.Unlock()
		}()

		// Deliberately not the request's context: the request that asked is
		// long gone by the time a two-hour film has been sampled, and cancelling
		// on it would mean the sheet is never built for anybody.
		//
		// It does need an end, though, and it had none. One encode runs at a
		// time, so a file ffmpeg cannot finish held that slot for the life of the
		// process and no other film ever got a strip -- a comfort failing quietly
		// for the whole library because of one bad file.
		ctx, cancel := context.WithTimeout(context.Background(), buildTimeout(duration))
		defer cancel()
		if m.jobs != nil {
			var release func()
			var err error
			ctx, release, err = m.jobs.BeginBackground(ctx)
			if err != nil {
				return
			}
			defer release()
		}

		select {
		case m.slot <- struct{}{}:
		case <-ctx.Done():
			if errors.Is(context.Cause(ctx), workload.ErrPreempted) {
				failed = false
			}
			return
		}
		defer func() { <-m.slot }()

		if err := work(ctx); err != nil {
			if errors.Is(context.Cause(ctx), workload.ErrPreempted) {
				failed = false
				m.log.Debug("seek preview yielded to playback", "source", source)
			} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				m.log.Warn("a seek preview took too long and was given up on",
					"source", source, "allowed", buildTimeout(duration))
			} else {
				m.log.Warn("a seek preview could not be built",
					"source", source, "error", err)
			}
			return
		}
		failed = false
	}()
}

// buildTimeout is how long one strip may take.
//
// Measured rather than picked: the 2 h 35 4K HDR file this was sized against
// takes 217 s, which is a keyframe-only decode of a 13.9 GB source plus a tone
// map. That is 2.3 per cent of its own running time, so the allowance is five per
// cent -- a little over twice what the real case needs.
//
// The floor exists because a short film is not proportionally cheaper: the cost
// is dominated by reading the file, not by its length. The ceiling exists
// because past a quarter of an hour the honest answer is that this file is not
// going to produce a strip, and the slot is worth more to the next film.
func buildTimeout(duration float64) time.Duration {
	// Whole seconds, so the value is predictable and testable rather than
	// carrying the float noise of a duration measured off a container.
	allowed := time.Duration(math.Round(duration/20)) * time.Second
	if allowed < 2*time.Minute {
		return 2 * time.Minute
	}
	if allowed > 15*time.Minute {
		return 15 * time.Minute
	}
	return allowed
}

func (m *Manager) build(ctx context.Context, key, source string, duration float64,
	colorTransfer string,
) error {
	binary, err := m.ffmpeg.Path(ctx)
	if err != nil {
		return err
	}

	interval := math.Max(minInterval, duration/float64(tiles))
	count := int(math.Floor(duration / interval))
	if count < 2 {
		return ErrUnavailable
	}
	if count > tiles {
		count = tiles
	}
	rows := (count + columns - 1) / columns

	sheet := filepath.Join(m.dir, key+".jpg")
	temp := filepath.Join(m.dir, key+".building.jpg")

	// -skip_frame nokey is what makes this affordable: only keyframes are
	// decoded, so a two-hour film is read in a fraction of the time a full
	// decode would take. The fps filter still lays them on an even time grid,
	// picking the nearest decoded frame to each mark, which is why one tile can
	// be trusted to mean one interval even though the source's keyframes are
	// not evenly spaced.
	filter := fmt.Sprintf("fps=1/%s,scale=-2:%d,tile=%dx%d",
		strconv.FormatFloat(interval, 'f', 3, 64), tileHeight, columns, rows)
	// The same conversion the player's re-encode does, for the same reason: a
	// strip cut from an HDR source and written straight to JPEG is grey. It sits
	// after the scale, where it costs almost nothing -- these frames are ninety
	// pixels tall by the time it runs.
	if stream.ToneMap(colorTransfer) {
		filter = fmt.Sprintf("fps=1/%s,scale=-2:%d,%s,tile=%dx%d",
			strconv.FormatFloat(interval, 'f', 3, 64), tileHeight,
			stream.ToneMapFilter, columns, rows)
	}

	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error",
		"-skip_frame", "nokey",
		"-i", source,
		"-an", "-sn", "-dn",
		"-frames:v", "1",
		"-vf", filter,
		"-qscale:v", "6",
		// Named rather than inferred. ffmpeg chooses a muxer from the file
		// extension and refuses when it does not recognise one, which is how
		// the first version of this failed on every file it was given.
		"-f", "image2",
		"-y", temp,
	)
	output := boundedio.NewTail(64 << 10)
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("ffmpeg: %w: %s", err, output.String())
	}

	if err := os.Rename(temp, sheet); err != nil {
		os.Remove(temp)
		return err
	}

	manifest := Manifest{
		Key:             key,
		IntervalSeconds: interval,
		Columns:         columns,
		Rows:            rows,
		Count:           count,
		TileHeight:      tileHeight,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	// The manifest is written last and is what Lookup reads, so a sheet is
	// never advertised before the picture behind it is complete.
	if err := os.WriteFile(filepath.Join(m.dir, key+".json"), data, 0o644); err != nil {
		return err
	}

	m.log.Info("built a seek preview", "source", source,
		"frames", count, "interval_seconds", interval)
	return nil
}

// buildClip encodes the card preview.
//
// -ss sits before -i on purpose: that is a keyframe seek, so the encoder jumps
// to the neighbourhood of the sample point instead of decoding everything up to
// it. The sample is six seconds of a two-hour film, and the difference between
// the two orders is the difference between a second and half an hour.
//
// The same tone map the strip uses runs here for the same reason - a clip cut
// from an HDR source and written straight out is grey - and it runs after the
// scale, where it costs almost nothing.
func (m *Manager) buildClip(ctx context.Context, key, source string, duration float64,
	colorTransfer string,
) error {
	binary, err := m.ffmpeg.Path(ctx)
	if err != nil {
		return err
	}

	start := ClipStart(duration)
	if start+float64(clipSeconds) > duration {
		// Never ask for more than the file holds: ffmpeg would simply stop at
		// the end, and a two-second clip is not worth the encode.
		if duration-start < 2 {
			return ErrUnavailable
		}
	}

	// One probe, two answers, taken from the same frames the clip is made of.
	//
	// HDR: a source that must be tone mapped, and asking the database is not
	// enough - it only knows once the file has been inspected, and a card preview
	// is built long before anybody plays the film. Measured on 20 September
	// 2026: the clip of an uninspected HDR remux kept its `bt2020/smpte2084`
	// tags and the webview refused it with `Format error`.
	//
	// Letterbox: most films are wider than the frame they are stored in, so the
	// source *contains* two black bars as picture. Measured: 122 of 480 rows in
	// the clip of the maintainer's own remux were black, `object-fit` cannot
	// remove them because they are content, and the maintainer saw exactly that.
	// Cropping them here is the same algorithm for a film, an episode and a
	// series, because all three end up in this function with a file.
	hdr, crop := m.probeSource(ctx, binary, source, start)
	if !hdr {
		hdr = stream.ToneMap(colorTransfer)
	}

	filters := make([]string, 0, 3)
	if crop != "" {
		filters = append(filters, "crop="+crop)
	}
	filters = append(filters, fmt.Sprintf("scale=-2:%d", clipHeight))
	if hdr {
		filters = append(filters, stream.ToneMapFilter)
	}
	scale := strings.Join(filters, ",")

	clip := filepath.Join(m.dir, key+".mp4")
	temp := filepath.Join(m.dir, key+".building.mp4")

	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatFloat(start, 'f', 3, 64),
		"-i", source,
		"-t", strconv.Itoa(clipSeconds),
		"-an", "-sn", "-dn",
		// One video stream, named. Without the mapping ffmpeg copies whatever
		// else it finds: this film's remux carries a `bin_data (text)` track, and
		// a clip whose MP4 holds a track the webview cannot name is a clip the
		// webview refuses.
		"-map", "0:v:0",
		"-vf", scale,
		// H.264 because the only thing that has to decode this is the webview
		// drawing the card, and H.264 is the one codec it always has.
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "30",
		"-pix_fmt", "yuv420p",
		// Labelled BT.709 whatever the source was, because the output is either
		// tone mapped already or was SDR to begin with. Inheriting the source's
		// tags is how a player is told to expect PQ in a file that holds SDR.
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		// The moov atom first, so a six-second clip starts playing before it has
		// finished arriving - the same reason a range request exists at all.
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y", temp,
	)
	output := boundedio.NewTail(64 << 10)
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("ffmpeg clip: %w: %s", err, output.String())
	}

	// Written whole or not at all: a half-written clip that plays for two
	// seconds is worse than the still it replaces.
	if err := os.Rename(temp, clip); err != nil {
		os.Remove(temp)
		return err
	}

	m.log.Info("built a card preview", "source", source, "start_seconds", start)
	return nil
}

// probeSource reads two things off the source's own frames: whether it is HDR,
// and whether it carries letterbox bars worth cropping.
//
// Deliberately not a probe of the file's name or of the metadata table. The
// transfer function is what ffmpeg reports after opening the source, which is
// the only answer true for the frames about to be encoded; the bars are measured
// by `cropdetect` on those same frames, at the same timestamp, so the crop
// describes what the clip will actually contain. Nothing is written, the read is
// a fraction of a second, and it runs once per clip.
//
// The crop is accepted only if it is plausible: a dark scene can persuade
// `cropdetect` that most of the picture is black, and cropping a film to a
// corner of itself is worse than keeping two bars. Half the frame and sixty-four
// rows are the floors, and `limit=24` is what stops a merely dim frame from
// counting as bar in the first place.
func (m *Manager) probeSource(ctx context.Context, binary, source string, at float64) (bool, string) {
	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner",
		"-ss", strconv.FormatFloat(at, 'f', 3, 64),
		"-i", source,
		"-frames:v", "24",
		// `format=yuv420p` first, and it is the whole reason this works on this
		// library: the film is 10-bit HDR, where limited-range black is 64, and
		// `limit=24` therefore sees no bars at all - measured, cropdetect
		// answered `crop=3840:2160:0:0` on the source and `crop=854:356:0:62` on
		// the very same frames once encoded to 8 bits. The conversion is for the
		// measurement only; nothing here is written.
		"-vf", "format=yuv420p,cropdetect=limit=24:round=2",
		"-f", "null",
		"-",
	)
	output := boundedio.NewTail(128 << 10)
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		// A source ffmpeg cannot open is neither an HDR nor a crop question; the
		// encode that follows will fail on its own terms and be remembered.
		return false, ""
	}

	reported := output.String()
	hdr := false
	lowered := strings.ToLower(reported)
	for _, marker := range []string{"smpte2084", "arib-std-b67", "bt2020"} {
		if strings.Contains(lowered, marker) {
			hdr = true
		}
	}

	// The last line cropdetect printed: it converges as it sees more frames, so
	// the newest answer is the most informed one.
	crop := ""
	shape := regexp.MustCompile(`crop=([0-9]+):([0-9]+):([0-9]+):([0-9]+)`)
	for _, match := range shape.FindAllStringSubmatch(reported, -1) {
		width, _ := strconv.Atoi(match[1])
		height, _ := strconv.Atoi(match[2])
		if width < 64 || height < 64 {
			continue
		}
		full := 0
		if size := regexp.MustCompile(`([0-9]+)x([0-9]+)`).FindStringSubmatch(reported); size != nil {
			w, _ := strconv.Atoi(size[1])
			h, _ := strconv.Atoi(size[2])
			full = w * h
		}
		if full > 0 && width*height < full/2 {
			continue
		}
		crop = match[1] + ":" + match[2] + ":" + match[3] + ":" + match[4]
	}
	return hdr, crop
}
