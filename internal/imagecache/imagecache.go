// Package imagecache stores TMDB posters and backdrops on disk and serves them
// from there.
//
// Images are fetched lazily, when something actually asks for one, rather than
// during a scan. A first scan of a large library would otherwise download
// thousands of files the user may never scroll past.
//
// TMDB image paths are content-addressed -- a given path always returns the
// same picture -- so a cached file never expires. What can change is which path
// a film points at, and that is governed by the metadata lifetime in
// internal/library, not here.
package imagecache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/Benitoow/theia-media/internal/tmdb"
)

// ErrUnavailable means the image cannot be produced: no TMDB client, an
// unsupported size, or a path that is not a TMDB image path.
var ErrUnavailable = errors.New("imagecache: image unavailable")

// allowedSizes is a whitelist rather than a passthrough. The size becomes a
// directory name, and TMDB rejects anything it does not recognise anyway.
var allowedSizes = map[string]bool{
	"w92": true, "w154": true, "w185": true, "w342": true, "w500": true,
	"w780": true, "w1280": true, "original": true,
}

// tmdbImagePath is what TMDB actually returns: a slash, a long opaque name, an
// extension. Anything else is rejected before it can reach the filesystem --
// this value arrives from a URL, and it ends up in a file path.
var tmdbImagePath = regexp.MustCompile(`^/?[A-Za-z0-9_-]{1,120}\.(?:jpg|jpeg|png|webp|svg)$`)

// Cache downloads images once and serves them from disk afterwards.
type Cache struct {
	dir    string
	client *tmdb.Client

	// Two viewers opening the same film at once must not both download the
	// same poster. Each path gets a lock for the duration of its download.
	mu    sync.Mutex
	locks map[string]*download
}

// download is a per-image lock with a reference count. The count matters:
// removing the entry while another goroutine is still queued on it would let a
// third arrival create a second lock and download the same file in parallel.
type download struct {
	mu      sync.Mutex
	waiters int
}

// New prepares the cache directory. client may be nil, in which case every
// lookup reports ErrUnavailable and nothing else breaks.
func New(dir string, client *tmdb.Client) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating the image cache directory %s: %w", dir, err)
	}
	return &Cache{dir: dir, client: client, locks: map[string]*download{}}, nil
}

// Path returns the local file holding the image, downloading it first if this
// is the first time anybody asked.
func (c *Cache) Path(ctx context.Context, size, imagePath string) (string, error) {
	if c.client == nil {
		return "", ErrUnavailable
	}
	if !allowedSizes[size] || !tmdbImagePath.MatchString(imagePath) {
		return "", ErrUnavailable
	}

	// Validated above, so this cannot escape the cache directory -- the pattern
	// admits no separators, no dots beyond the extension, and no empty name.
	name := filepath.Base(imagePath)
	local := filepath.Join(c.dir, size, name)

	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	unlock := c.lockFor(local)
	defer unlock()

	// Another request may have finished the download while we waited.
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	data, _, err := c.client.FetchImage(ctx, size, imagePath)
	if err != nil {
		if errors.Is(err, tmdb.ErrNotFound) {
			return "", ErrUnavailable
		}
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return "", fmt.Errorf("creating the image cache directory: %w", err)
	}

	// Write to a temporary file and rename, so a request interrupted halfway
	// cannot leave a truncated JPEG behind that every later request then serves
	// happily from cache.
	tmp, err := os.CreateTemp(filepath.Dir(local), ".partial-*")
	if err != nil {
		return "", fmt.Errorf("caching image: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("caching image: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("caching image: %w", err)
	}
	if err := os.Rename(tmpName, local); err != nil {
		return "", fmt.Errorf("caching image: %w", err)
	}
	return local, nil
}

// lockFor serialises downloads of one image and returns the release function.
func (c *Cache) lockFor(key string) func() {
	c.mu.Lock()
	d, ok := c.locks[key]
	if !ok {
		d = &download{}
		c.locks[key] = d
	}
	d.waiters++
	c.mu.Unlock()

	d.mu.Lock()
	return func() {
		d.mu.Unlock()

		// Drop the entry only once nobody is queued behind it; the map would
		// otherwise grow by one entry per distinct image, forever.
		c.mu.Lock()
		d.waiters--
		if d.waiters == 0 {
			delete(c.locks, key)
		}
		c.mu.Unlock()
	}
}
