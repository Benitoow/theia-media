package api

import (
	"net/http"
	"strconv"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

// Where a seek actually lands, asked separately from the stream itself.
//
// A remux copies the picture, and a copy starts on a keyframe. Decision 90 made
// the sound start there too, which is what keeps a seeked film together -- and
// leaves the player counting from the moment it asked for rather than the one it
// received, up to a keyframe interval later. It then saves that number as the
// resume position.
//
// The player could not answer this and neither could the stream: the fragmented
// MP4 muxer rebases every timestamp to zero on the way out, verified, including
// under -copyts and -avoid_negative_ts disabled. So it is a question with its own
// route.
//
// It is deliberately *not* answered before the stream. Measured at about 200 ms
// on a 13.9 GB file, which is small but not nothing, and making the picture wait
// for a clock correction would be paying in the one currency a seek cannot spare.
// The player asks for both at once and adjusts when this arrives.

type seekStartResponse struct {
	// Requested is the time the player asked for, echoed so a late answer that
	// no longer matches the current playback can be discarded.
	Requested float64 `json:"requested"`

	// Start is where the stream really begins, in seconds of film.
	Start float64 `json:"start"`
}

func (s *Server) handleMovieFileSeekStart(w http.ResponseWriter, r *http.Request) {
	movie, file, ok := s.movieFileForStream(w, r)
	if !ok {
		return
	}

	raw := r.URL.Query().Get("t")
	requested, err := strconv.ParseFloat(raw, 64)
	if err != nil || requested <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_position")
		return
	}

	// The same promise /info and the encoder probe make: asking a question about
	// a file must never be the thing that downloads eighty megabytes.
	if s.ffmpeg == nil || !ffmpeg.Supported() || !s.ffmpeg.Available() {
		writeJSONError(w, http.StatusNotFound, "seek_start_unavailable")
		return
	}

	start, err := s.ffmpeg.KeyframeAt(r.Context(), file.Path, requested)
	if err != nil {
		// Not an error the interface has to explain. The clock stays as
		// optimistic as it was before this route existed, which is a comfort
		// missing rather than a playback broken.
		s.log.Debug("could not locate the keyframe for a seek",
			"film_id", movie.ID, "file_id", file.ID, "requested", requested, "error", err)
		writeJSONError(w, http.StatusNotFound, "seek_start_unavailable")
		return
	}

	// A keyframe after the request would mean the answer is not the one the
	// stream used; only ever correct backwards.
	if start > requested {
		start = requested
	}
	writeJSON(w, http.StatusOK, seekStartResponse{Requested: requested, Start: start})
}
