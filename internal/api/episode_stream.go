package api

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/subtitles"
)

// The episode side of the delivery pair. Its tables and routes stay distinct
// from the film side (decision 39); the shared bodies are in stream_twin.go.

func (s *Server) handleEpisodeFileStreamInfo(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamInfo(w, r, s.episodeTwin(item, file))
}

func (s *Server) handleEpisodeFileStreamDirect(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamDirect(w, r, s.episodeTwin(item, file))
}

func (s *Server) handleEpisodeFileStreamRemux(w http.ResponseWriter, r *http.Request) {
	item, file, ok := s.episodeFileForStream(w, r)
	if !ok {
		return
	}
	s.serveStreamRemux(w, r, s.episodeTwin(item, file))
}

// episodeTwin carries the episode side's identity and the library calls that
// differ from the film side. The episode route does not validate ?h= up
// front; the delivery service reaches the same check later, which is the
// asymmetry the contract tests pin.
func (s *Server) episodeTwin(item library.EpisodeItem, file library.EpisodeFile) streamTwin {
	return streamTwin{
		id:        item.ID,
		episodeID: item.ID,
		fileID:    file.ID,
		label:     "episode",
		media:     file.Media,
		extension: file.Extension,
		progress:  item.Progress,
		path:      file.Path,
		audioTrack: func(ctx context.Context, audioID int64) (library.AudioTrack, error) {
			return s.lib.EpisodeAudioTrack(ctx, item.ID, file.ID, audioID)
		},
		syncSubtitles: func(ctx context.Context) library.FileMedia {
			// See the film side: sidecars are re-read once per playback.
			s.syncSidecars(file.Path, file.ID, func(id int64, found []subtitles.Sidecar) error {
				return s.lib.SyncEpisodeFileSubtitles(ctx, id, found)
			})
			if reloaded, err := s.lib.GetEpisodeFile(ctx, item.ID, file.ID); err == nil {
				file.Media.SubtitleTracks = reloaded.Media.SubtitleTracks
			}
			return file.Media
		},
		saveMeasured: func(ctx context.Context, media library.FileMedia) (library.FileMedia, error) {
			saved, err := s.lib.SaveEpisodeFileMedia(ctx, item.ID, file.ID, media)
			if err != nil {
				return library.FileMedia{}, err
			}
			return saved.Media, nil
		},
		markUnreadable: func(ctx context.Context) error {
			return s.lib.MarkEpisodeFileMediaError(ctx, item.ID, file.ID)
		},
		fallbackDuration: func() float64 { return episodeRuntimeSeconds(item) },
	}
}

func episodeRuntimeSeconds(item library.EpisodeItem) float64 {
	minutes := 0
	for _, episode := range item.Episodes {
		minutes += episode.Metadata.RuntimeMinutes
	}
	return float64(minutes) * 60
}

func (s *Server) episodeFileForStream(w http.ResponseWriter, r *http.Request) (library.EpisodeItem, library.EpisodeFile, bool) {
	itemID, ok := positivePathID(w, r, "id", "invalid_episode_id")
	if !ok {
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	fileID, ok := positivePathID(w, r, "file_id", "invalid_file_id")
	if !ok {
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	profileID, ok := s.resolveProfile(w, r)
	if !ok {
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}

	item, err := s.lib.GetEpisodeItem(r.Context(), profileID, itemID)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeItem):
		writeJSONError(w, http.StatusNotFound, "episode_not_found")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	case err != nil:
		s.log.Error("reading an episode for a file request failed", "episode_id", itemID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "episode_unavailable")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	file, err := s.lib.GetEpisodeFile(r.Context(), itemID, fileID)
	switch {
	case errors.Is(err, library.ErrNoSuchEpisodeFile):
		writeJSONError(w, http.StatusNotFound, "file_not_found")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	case err != nil:
		s.log.Error("reading an episode file failed",
			"episode_id", itemID, "file_id", fileID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "file_unavailable")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	resolved, inside := s.resolvedLibraryPath(file.Path)
	if !inside {
		writeJSONError(w, http.StatusForbidden, "file_outside_library")
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	file.Path = resolved
	if _, err := os.Stat(file.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
		} else {
			writeJSONError(w, http.StatusInternalServerError, "media_file_unreadable")
		}
		return library.EpisodeItem{}, library.EpisodeFile{}, false
	}
	return item, file, true
}
