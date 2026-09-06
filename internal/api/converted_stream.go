package api

import (
	"bufio"
	"io"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

// Films and episodes share the same delivery contract. Identity resolution and
// persistence remain in their handlers; encoder selection and pipes do not.
func (s *Server) serveConvertedFile(w http.ResponseWriter, r *http.Request, path string, media library.FileMedia, decision stream.Decision, selected *library.AudioTrack) {
	height, requested, ok := requestedHeight(w, r)
	if !ok {
		return
	}
	start := 0.0
	if value := r.URL.Query().Get("t"); value != "" {
		var err error
		start, err = strconv.ParseFloat(value, 64)
		if err != nil || start < 0 || math.IsNaN(start) || math.IsInf(start, 0) {
			writeJSONError(w, 400, "invalid_position")
			return
		}
	}
	var index *int
	if selected != nil {
		value := selected.StreamIndex
		index = &value
	}
	binary, err := s.ffmpeg.Path(r.Context())
	if err != nil {
		writeJSONError(w, 503, "ffmpeg_unavailable")
		return
	}
	var args []string
	if decision.Mode == stream.ModeUnsupported || requested || forcedTranscode(r) {
		encoder, available := s.ffmpeg.Capabilities(r.Context()).Best()
		if !available {
			writeJSONError(w, 415, "video_transcode_required")
			return
		}
		s.transcodes.setKind(encoder.Kind)
		release, available := s.transcodes.acquire(stream.ToneMap(media.Video.ColorTransfer))
		if !available {
			w.Header().Set("Retry-After", "1")
			writeJSONError(w, 503, "transcode_busy")
			return
		}
		defer release()
		decision.Mode = stream.ModeTranscode
		args = stream.TranscodeArgs(path, decision, start, stream.TranscodeOptions{
			Encoder: encoder.Name, HWAccel: s.ffmpeg.HardwareDecoder(r.Context()), Height: height,
			SourceHeight: media.Video.Height, ColorTransfer: media.Video.ColorTransfer, AudioStreamIndex: index,
		})
	} else if index != nil {
		args = stream.RemuxArgsForAudio(path, decision, start, *index)
	} else {
		args = stream.RemuxArgs(path, decision, start)
	}
	s.log.Info("starting media stream", "mode", decision.Mode, "start", start, "height", height, "audio", decision.Audio)
	cmd := exec.CommandContext(r.Context(), binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeJSONError(w, 500, "stream_start_failed")
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		writeJSONError(w, 500, "stream_start_failed")
		return
	}
	// Do not send a successful video response for an encoder that never opens.
	reader := bufio.NewReader(stdout)
	if _, err := reader.Peek(1); err != nil {
		_ = cmd.Wait()
		s.log.Warn("media encoder failed before output", "detail", stderr.String())
		writeJSONError(w, 502, "stream_encode_failed")
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Accept-Ranges", "none")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, reader)
	if copyErr != nil {
		_ = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if err != nil && r.Context().Err() == nil && copyErr == nil {
		s.log.Warn("media encoder ended early", "error", err, "detail", stderr.String())
	}
}
