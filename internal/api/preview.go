package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/preview"
)

// Seek previews.
//
// The strip of frames a player shows under the cursor while somebody drags the
// bar. It is a comfort and is treated as one everywhere: three states, none of
// them an error the interface has to explain. Ready, not yet, or never — and
// the player simply draws no preview for the last two.
//
// "Never" covers the case that matters most: no ffmpeg on disk. Asking for a
// preview must not be the thing that downloads eighty megabytes, which is the
// promise M1 made for /info and decision 58 restated for the encoder probe.

type previewResponse struct {
	// State is "ready" or "building". Anything permanent is a 404, because
	// there is nothing for the player to wait for.
	State string `json:"state"`

	Manifest *preview.Manifest `json:"manifest,omitempty"`

	// SheetURL is where the picture is, once there is one.
	SheetURL string `json:"sheet_url,omitempty"`
}

// handleMoviePreview answers for a film's primary file.
func (s *Server) handleMoviePreview(w http.ResponseWriter, r *http.Request) {
	movie, ok := s.movieForStream(w, r)
	if !ok {
		return
	}
	duration, transfer := s.moviePrimaryFacts(r, movie.ID)
	s.writePreview(w, r, movie.Path, movie.SizeBytes, movie.ModifiedAt, duration, transfer)
}

// handleMovieFilePreview answers for one chosen file of a film.
func (s *Server) handleMovieFilePreview(w http.ResponseWriter, r *http.Request) {
	_, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	s.writePreview(w, r, file.Path, file.SizeBytes, file.ModifiedAt,
		file.Media.DurationSeconds, colorTransferOf(file.Media))
}

// handleEpisodeFilePreview answers for one file of an episode.
func (s *Server) handleEpisodeFilePreview(w http.ResponseWriter, r *http.Request) {
	_, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	s.writePreview(w, r, file.Path, file.SizeBytes, file.ModifiedAt,
		file.Media.DurationSeconds, colorTransferOf(file.Media))
}

// handlePreviewSheet serves a built sheet by its key.
//
// One route for every kind of item, because the key is a digest of the file and
// says nothing about whether that file is a film or an episode. The key is
// validated against a hex pattern inside the package before it touches the
// filesystem.
func (s *Server) handlePreviewSheet(w http.ResponseWriter, r *http.Request) {
	if s.previews == nil {
		writeJSONError(w, http.StatusNotFound, "previews are unavailable")
		return
	}

	path, err := s.previews.SheetPath(r.PathValue("key"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "no such preview")
		return
	}

	// The key changes when the file does, so the sheet behind one URL never
	// changes and may be kept for as long as the browser likes.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, path)
}

func (s *Server) writePreview(w http.ResponseWriter, r *http.Request,
	path string, size int64, modified time.Time, duration float64, colorTransfer string,
) {
	if s.previews == nil {
		writeJSONError(w, http.StatusNotFound, "previews are unavailable")
		return
	}

	key := preview.Key(path, size, modified.Unix())
	manifest, err := s.previews.Lookup(r.Context(), key, path, duration, colorTransfer)
	switch {
	case errors.Is(err, preview.ErrNotReady):
		// Not an error and not a failure: something is being built, and asking
		// again in a while is the whole protocol.
		writeJSON(w, http.StatusOK, previewResponse{State: "building"})
		return
	case err != nil:
		writeJSONError(w, http.StatusNotFound, "no preview for this file")
		return
	}

	writeJSON(w, http.StatusOK, previewResponse{
		State:    "ready",
		Manifest: &manifest,
		SheetURL: "/api/previews/" + manifest.Key,
	})
}

// moviePrimaryFacts is what the legacy film-level preview route needs to know
// about a film without being given a file: how long it runs, and whether its
// picture has to be tone mapped.
//
// The duration decides whether a preview is worth building and how far apart its
// frames sit. Three sources, in order of how much they are worth: a duration the
// player measured and saved, then one measured off the file by an inspection,
// then TMDB's runtime. The last is in whole minutes and is only ever a few
// seconds out, which moves a tile by a fraction of one interval.
//
// The transfer function has only one source, because it is a measurement: the
// primary file, if it has been inspected. Both come out of one read.
func (s *Server) moviePrimaryFacts(r *http.Request, id int64) (float64, string) {
	profileID, err := s.profileID(r)
	if err != nil {
		return 0, ""
	}
	movie, err := s.lib.Get(r.Context(), profileID, id)
	if err != nil {
		return 0, ""
	}

	duration, transfer := 0.0, ""
	for _, file := range movie.Files {
		if !file.IsPrimary {
			continue
		}
		transfer = colorTransferOf(file.Media)
		duration = file.Media.DurationSeconds
		break
	}
	switch {
	case movie.Progress.DurationSeconds > 0:
		duration = movie.Progress.DurationSeconds
	case duration <= 0:
		duration = float64(movie.Metadata.Runtime) * 60
	}
	return duration, transfer
}

// colorTransferOf reports a measured transfer function, and nothing else. A file
// that has not been inspected says nothing rather than guessing from its name,
// which is the same rule the file chooser follows.
func colorTransferOf(media library.FileMedia) string {
	if media.Status != library.MediaOK || media.Video == nil {
		return ""
	}
	return media.Video.ColorTransfer
}
