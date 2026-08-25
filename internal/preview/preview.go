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
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/stream"
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
}

// New prepares the cache directory.
func New(dir string, binary Binary, log *slog.Logger) (*Manager, error) {
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

	m.start(key, source, duration, colorTransfer)
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

// start kicks off one build, unless that key is already being built.
func (m *Manager) start(key, source string, duration float64, colorTransfer string) {
	m.mu.Lock()
	if m.building[key] || m.failed[key] {
		m.mu.Unlock()
		return
	}
	m.building[key] = true
	m.mu.Unlock()

	go func() {
		failed := true
		defer func() {
			m.mu.Lock()
			delete(m.building, key)
			if failed {
				m.failed[key] = true
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

		m.slot <- struct{}{}
		defer func() { <-m.slot }()

		if err := m.build(ctx, key, source, duration, colorTransfer); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("ffmpeg: %w: %s", err, string(output))
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
