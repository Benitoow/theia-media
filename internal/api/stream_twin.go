package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/playback"
	"github.com/Benitoow/theia-media/internal/stream"
)

// streamTwin is the identity half of a film/episode pair. The delivery policy
// is shared -- one /info, one direct play, one remux -- while tables, routes
// and identity stay separate (decision 39), so what the shared bodies need
// from their side of the pair travels in here: the ids the response reports,
// the library calls that differ, and the one fallback that is not the same
// arithmetic on both sides.
//
// The func fields are the twins' whole extent. Nothing here decides anything
// about identity; the adapters keep their route families and their error
// codes, and the playback service keeps the delivery policy.
type streamTwin struct {
	// id is the id the response reports: the film id, or the episode item id.
	id int64
	// movieID is the owning film id on the film side, zero on the episode
	// side -- the response omits it either way.
	movieID int64
	// episodeID is the episode item id on the episode side, zero on the film
	// side.
	episodeID int64
	fileID    int64

	// label names the side in logs: "film" or "episode".
	label string

	media library.FileMedia
	// extension is the file's own, and the container answer when the
	// inspection never recorded one.
	extension string
	// progress is the row the response carries.
	progress library.Progress
	// path is the resolved, contained path of the media file.
	path string

	// audioTrack resolves one requested track under this side's tables.
	audioTrack func(ctx context.Context, audioID int64) (library.AudioTrack, error)
	// syncSubtitles records the sidecars beside the file and answers the
	// media as the row now reads, subtitle tracks refreshed.
	syncSubtitles func(ctx context.Context) library.FileMedia
	// saveMeasured persists a fresh inspection and answers it as stored, with
	// the ids the database assigned.
	saveMeasured func(ctx context.Context, media library.FileMedia) (library.FileMedia, error)
	// markUnreadable records a file the probe could not open.
	markUnreadable func(ctx context.Context) error
	// fallbackDuration resolves a duration when the file has none.
	fallbackDuration func() float64

	// validatesHeight marks the side that checks ?h= before anything but the
	// audio parameter -- the film route, as the contract tests found it. The
	// episode route reaches the same validation inside the delivery service,
	// only later; that ordering difference is pinned, not accidental.
	validatesHeight bool
}

// serveStreamInfo answers how a file will be delivered, for either side of
// the twin pair. The decision is made from the container alone -- asking a
// question must never run, let alone download, ffmpeg (the M1 promise) -- and
// upgraded by the planner once the file has been measured.
func (s *Server) serveStreamInfo(w http.ResponseWriter, r *http.Request, twin streamTwin) {
	audioID, audioRequested, ok := requestedAudioID(w, r)
	if !ok {
		return
	}

	var selected *library.AudioTrack
	if audioRequested {
		if twin.media.Status != library.MediaOK {
			writeJSONError(w, http.StatusConflict, "media_not_inspected")
			return
		}
		track, err := twin.audioTrack(r.Context(), audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			s.log.Error("reading an audio track failed",
				"parent_id", twin.id, "file_id", twin.fileID, "track_id", audioID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "audio_track_unavailable")
			return
		}
		selected = &track
	}

	capabilities := s.videoCapabilities(r.Context())
	decision, reasonCode := playback.InfoDecision(twin.path, twin.media, selected, audioRequested, capabilities.Available)

	duration := twin.media.DurationSeconds
	if duration <= 0 {
		duration = twin.progress.DurationSeconds
	}
	if duration <= 0 {
		duration = twin.fallbackDuration()
	}
	container := twin.media.Container
	if container == "" {
		container = twin.extension
	}
	// Asked once per playback, which is what keeps a `.srt` dropped in this
	// afternoon offered this evening without a rescan.
	media := twin.syncSubtitles(r.Context())

	// V2-M6. A picture no browser decodes stops being a refusal when this
	// machine has an encoder that runs.
	ladder := qualityLadder(media, decision, capabilities.Available)
	height, frameRate := 0, 0.0
	if media.Video != nil {
		height = media.Video.Height
		frameRate = media.Video.FrameRate
	}

	progress := twin.progress
	writeJSON(w, http.StatusOK, streamInfoResponse{
		ID:              twin.id,
		MovieID:         twin.movieID,
		EpisodeID:       twin.episodeID,
		FileID:          twin.fileID,
		AudioTrackID:    audioID,
		Mode:            string(decision.Mode),
		ReasonCode:      reasonCode,
		Container:       container,
		MediaStatus:     media.Status,
		VideoRisky:      decision.VideoRisky,
		ToneMap:         media.Video != nil && stream.ToneMap(media.Video.ColorTransfer),
		VideoCodec:      videoCodecOf(media),
		FFmpegReady:     s.ffmpeg != nil && s.ffmpeg.Available(),
		FFmpegSupported: ffmpeg.Supported(),
		DurationSeconds: duration,
		Progress:        &progress,
		AudioTracks:     media.AudioTracks,
		SubtitleTracks:  media.SubtitleTracks,
		Height:          height,
		FrameRate:       frameRate,
		Qualities:       ladder,
		Transcode:       &capabilities,
	})
}

// serveStreamDirect serves the file untouched, for either side of the pair.
func (s *Server) serveStreamDirect(w http.ResponseWriter, r *http.Request, twin streamTwin) {
	if r.URL.Query().Has("audio") {
		writeJSONError(w, http.StatusBadRequest, "audio_selection_requires_remux")
		return
	}

	endPlayback, admitted := s.beginPlayback(w)
	if !admitted {
		return
	}
	defer endPlayback()
	s.serveDirectContent(w, r, twin.path, twin.label, twin.movieID, twin.fileID)
}

// serveDirectContent opens the file and hands it to http.ServeContent, which
// does the whole of HTTP range handling -- what makes the browser's seek bar
// work: it asks for the byte range around the timestamp and gets a 206 back.
// Both the per-file routes and the legacy route end here.
func (s *Server) serveDirectContent(w http.ResponseWriter, r *http.Request, path, label string, parentID, fileID int64) {
	opened, err := os.Open(path)
	if err != nil {
		s.log.Warn("opening a media file for direct play failed",
			"kind", label, "parent_id", parentID, "file_id", fileID,
			"path", path, "error", err)
		writeJSONError(w, http.StatusNotFound, "media_file_unavailable")
		return
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "media_file_unreadable")
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(path))
	// Private: this is one household's media, and no proxy has any business
	// keeping a copy of it.
	w.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), opened)
}

// serveStreamRemux rewraps -- or re-encodes -- for either side of the pair.
// The film route validates ?h= before this point, the episode route does not:
// the asymmetry the contract tests pin, kept by leaving the check in the film
// adapter.
func (s *Server) serveStreamRemux(w http.ResponseWriter, r *http.Request, twin streamTwin) {
	audioID, audioRequested, ok := requestedAudioID(w, r)
	if !ok {
		return
	}
	// The film side validates the height here, before anything but the audio
	// parameter; the episode side reaches the same validation inside the
	// delivery service, only later. The asymmetry the contract tests pin.
	if twin.validatesHeight {
		if _, _, derr := playback.RequestedHeight(r); derr != nil {
			s.writeDeliveryError(w, derr)
			return
		}
	}
	if s.ffmpeg == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
		return
	}

	endPlayback, admitted := s.beginPlayback(w)
	if !admitted {
		return
	}
	defer endPlayback()

	// SubtitlesScanned is the third condition, and it only ever fires once per
	// file: a library inspected before subtitles existed has every measurement
	// except that one, and re-probing here costs a single ffmpeg run on a path
	// that was about to start one anyway.
	if twin.media.Status != library.MediaOK || twin.media.Video == nil || !twin.media.SubtitlesScanned {
		releaseWork := s.beginCostlyWork()
		info, err := s.ffmpeg.Probe(r.Context(), twin.path)
		releaseWork()
		switch {
		case errors.Is(err, ffmpeg.ErrUnsupportedPlatform):
			writeJSONError(w, http.StatusNotImplemented, "ffmpeg_unsupported")
			return
		case errors.Is(err, ffmpeg.ErrMediaUnreadable):
			_ = twin.markUnreadable(r.Context())
			s.log.Warn("probing a media file failed",
				"kind", twin.label, "parent_id", twin.movieID, "file_id", twin.fileID,
				"path", twin.path, "error", err)
			writeJSONError(w, http.StatusUnsupportedMediaType, "media_unreadable")
			return
		case err != nil:
			s.log.Error("ffmpeg could not inspect a media file",
				"kind", twin.label, "parent_id", twin.movieID, "file_id", twin.fileID, "error", err)
			writeJSONError(w, http.StatusServiceUnavailable, "ffmpeg_unavailable")
			return
		}
		stored, err := twin.saveMeasured(r.Context(), measuredFileMedia(info))
		if err != nil {
			s.log.Error("saving a file inspection failed",
				"kind", twin.label, "parent_id", twin.movieID, "file_id", twin.fileID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "media_inspection_not_saved")
			return
		}
		twin.media = stored
	}

	var selected *library.AudioTrack
	if audioRequested {
		track, err := twin.audioTrack(r.Context(), audioID)
		switch {
		case errors.Is(err, library.ErrNoSuchAudioTrack):
			writeJSONError(w, http.StatusNotFound, "audio_track_not_found")
			return
		case err != nil:
			s.log.Error("reading a selected audio track failed",
				"parent_id", twin.movieID, "file_id", twin.fileID, "track_id", audioID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "audio_track_unavailable")
			return
		}
		selected = &track
	}

	decision, selected := playback.StreamDecision(twin.path, twin.media, selected, audioRequested)
	if derr := s.playback.ServeConverted(w, r, twin.path, twin.media, decision, selected); derr != nil {
		s.writeDeliveryError(w, derr)
	}
}

// videoCodecOf is the measured codec, or empty when the file has not been
// inspected. Lowercase, because it is a key the browser stores its own verdict
// under and "HEVC" and "hevc" must not become two answers.
func videoCodecOf(media library.FileMedia) string {
	if media.Status != library.MediaOK || media.Video == nil {
		return ""
	}
	return strings.ToLower(media.Video.Codec)
}
