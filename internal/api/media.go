package api

import (
	"errors"
	"net/http"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
)

// handleInspectMovieFile is the explicit boundary that may prepare/download
// ffmpeg. GET detail and GET stream info remain cheap and never trigger it.
func (s *Server) handleInspectMovieFile(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}
	if s.ffmpeg == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	info, err := s.ffmpeg.Probe(r.Context(), file.Path)
	switch {
	case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
		writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
		return
	case errors.Is(err, ffmpeg.ErrMediaUnreadable):
		if markErr := s.lib.MarkFileMediaError(r.Context(), movie.ID, file.ID); markErr != nil {
			s.log.Warn("recording a failed media inspection failed",
				"film_id", movie.ID, "file_id", file.ID, "error", markErr)
		}
		s.log.Warn("inspecting a film file failed",
			"film_id", movie.ID, "file_id", file.ID, "path", file.Path, "error", err)
		writeJSONError(w, http.StatusUnsupportedMediaType, "media_unreadable")
		return
	case err != nil:
		s.log.Error("ffmpeg could not inspect a film file",
			"film_id", movie.ID, "file_id", file.ID, "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	measured := measuredFileMedia(info)
	saved, err := s.lib.SaveFileMedia(r.Context(), movie.ID, file.ID, measured)
	if err != nil {
		s.log.Error("saving a media inspection failed",
			"film_id", movie.ID, "file_id", file.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "media_inspection_not_saved")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func measuredFileMedia(info ffmpeg.MediaInfo) library.FileMedia {
	media := library.FileMedia{
		Status:          library.MediaOK,
		Container:       info.Container,
		DurationSeconds: info.Seconds,
		Video: &library.VideoStream{
			StreamIndex: info.Video.StreamIndex,
			Codec:       info.Video.Codec,
			Width:       info.Video.Width,
			Height:      info.Video.Height,
		},
		AudioTracks:    make([]library.AudioTrack, 0, len(info.AudioStreams)),
		SubtitleTracks: make([]library.SubtitleTrack, 0, len(info.SubtitleStreams)),
	}
	for _, track := range info.AudioStreams {
		media.AudioTracks = append(media.AudioTracks, library.AudioTrack{
			StreamIndex: track.StreamIndex,
			Codec:       track.Codec,
			Language:    track.Language,
			Title:       track.Title,
			Channels:    track.Channels,
			IsDefault:   track.Default,
		})
	}
	for _, track := range info.SubtitleStreams {
		// Bitmap tracks are recorded alongside the text ones. Decision 3 refuses
		// to render them, not to admit they exist: a rip whose only subtitles are
		// PGS should say so rather than look like a film with none.
		index := track.StreamIndex
		media.SubtitleTracks = append(media.SubtitleTracks, library.SubtitleTrack{
			StreamIndex: &index,
			Codec:       track.Codec,
			Language:    track.Language,
			Title:       track.Title,
			IsDefault:   track.Default,
			IsForced:    track.Forced,
		})
	}
	return media
}
