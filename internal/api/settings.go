package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/imagecache"
)

// TMDBAttribution is required by the TMDB terms of use and must appear
// somewhere a user can see it. The wording is theirs and is not ours to
// paraphrase.
const TMDBAttribution = "This product uses the TMDB API but is not endorsed or certified by TMDB."

type tmdbSettings struct {
	Configured bool `json:"configured"`

	// Where the key came from: settings, built-in, config.local.json, or none.
	// Reported so that "why is it using the wrong key" has an answer that does
	// not involve reading source code. The key itself is never sent.
	Source config.KeySource `json:"source"`

	Attribution string `json:"attribution"`
}

type settingsResponse struct {
	Version      string       `json:"version"`
	Port         int          `json:"port"`
	Hostname     string       `json:"hostname"`
	DataDir      string       `json:"data_dir"`
	LibraryPaths []string     `json:"library_paths"`
	TMDB         tmdbSettings `json:"tmdb"`
}

// handleSettings reports the running configuration. It never includes the API
// key, only whether there is one and where it came from.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	tmdbInfo := tmdbSettings{
		Configured:  s.keySource != config.KeyMissing,
		Source:      s.keySource,
		Attribution: TMDBAttribution,
	}
	writeJSON(w, http.StatusOK, settingsResponse{
		Version:      s.version,
		Port:         s.cfg.Port,
		Hostname:     s.cfg.Hostname,
		DataDir:      s.cfg.Dir(),
		LibraryPaths: s.cfg.LibraryPaths,
		TMDB:         tmdbInfo,
	})
}

// handleImage serves a cached TMDB poster or backdrop, downloading it on first
// request. Both path segments are validated inside the cache before they touch
// the filesystem.
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	if s.images == nil {
		writeJSONError(w, http.StatusNotFound, "images are unavailable")
		return
	}

	size := r.PathValue("size")
	name := r.PathValue("name")

	local, err := s.images.Path(r.Context(), size, "/"+strings.TrimPrefix(name, "/"))
	switch {
	case errors.Is(err, imagecache.ErrUnavailable):
		writeJSONError(w, http.StatusNotFound, "no such image")
		return
	case err != nil:
		s.log.Warn("fetching an image failed", "size", size, "name", name, "error", err)
		writeJSONError(w, http.StatusBadGateway, "the image could not be fetched")
		return
	}

	file, err := os.Open(local)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "no such image")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "the image could not be read")
		return
	}

	// A TMDB image path always returns the same picture, so once it is on disk
	// it can be cached in the browser indefinitely.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, filepath.Base(local), info.ModTime(), file)
}

// libraryRoots is the list of watched folders to act on right now.
//
// The watcher owns that list while it is running, because it reads it from a
// goroutine while the settings handler writes it from an HTTP request. Falling
// back to the configuration keeps the server usable without one, which is how
// most of the tests build it.
func (s *Server) libraryRoots() []string {
	if s.watcher != nil {
		return s.watcher.Roots()
	}
	return s.cfg.LibraryPaths
}
