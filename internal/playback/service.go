package playback

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/boundedio"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

// Binary is the ffmpeg manager as execution needs it. A small interface,
// defined here because it is consumed here, so a test can hand the service a
// fake: the real Manager re-hashes every binary it is pointed at and would
// download one that is missing, which is not a thing a unit test may trigger.
type Binary interface {
	Available() bool
	Path(ctx context.Context) (string, error)
	Capabilities(ctx context.Context) ffmpeg.Capabilities
	HardwareDecoder(ctx context.Context) string
}

// Workload gives interactive delivery its priority over background work.
type Workload interface {
	BeginInteractive() func()
}

// Options builds a Service.
type Options struct {
	Binary   Binary
	Workload Workload // nil where there is nothing to preempt
	Logger   *slog.Logger

	// StreamLimit is the remux ceiling; zero selects the default. Exists for
	// tests, which want a ceiling low enough to reach.
	StreamLimit int
}

// Service owns the delivery policy at the moment it becomes processes: it is
// the only place in Theia that spawns ffmpeg for a viewer, the only place that
// decides what a converted stream may cost, and the registry's owner at
// shutdown.
//
// Films and episodes keep their routes, tables and identity resolution
// (decision 39); what they share -- encoder selection, pipes, headers, the
// kill on a broken copy, the session registry -- lives here, once.
type Service struct {
	binary     Binary
	workload   Workload
	log        *slog.Logger
	transcodes *transcodeLimiter
	sessions   *Sessions

	// transcodeCeilingOnce derives the transcode budget from the encoder
	// probe once per process. The budget used to be re-derived on every
	// request that looked at it; the probe answers once, so the ceiling does
	// too.
	transcodeCeilingOnce sync.Once
}

// NewService builds the service.
func NewService(opts Options) *Service {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{
		binary:     opts.Binary,
		workload:   opts.Workload,
		log:        log,
		transcodes: newTranscodeLimiter(),
		sessions:   NewSessions(opts.StreamLimit, log),
	}
}

// DeliveryError is a refusal the adapter turns into the JSON response the
// interface already knows. A nil from ServeConverted means the response has
// been started -- bytes or headers are on their way -- and there is nothing
// left to answer with.
type DeliveryError struct {
	Status int
	Code   string

	// RetryAfter, when set, travels as a Retry-After header in seconds.
	RetryAfter int
}

// ServeConverted delivers a file the browser cannot read as it is: rewrapped
// or re-encoded, streamed as fragmented MP4.
//
// Films and episodes arrive here after their adapters resolved identity and
// persisted the inspection; everything from this point is shared. The error
// return is non-nil only while nothing has been written to the response.
func (s *Service) ServeConverted(w http.ResponseWriter, r *http.Request, path string,
	media library.FileMedia, decision stream.Decision, selected *library.AudioTrack,
) *DeliveryError {
	if s.workload != nil {
		release := s.workload.BeginInteractive()
		defer release()
	}
	height, requested, derr := RequestedHeight(r)
	if derr != nil {
		return derr
	}
	start, derr := RequestedStart(r)
	if derr != nil {
		return derr
	}
	binary, err := s.binary.Path(r.Context())
	if err != nil {
		return &DeliveryError{Status: http.StatusServiceUnavailable, Code: "ffmpeg_unavailable"}
	}
	var sourceHeight int
	var sourceFrameRate float64
	var sourceVideoCodec, sourceColorTransfer string
	if media.Video != nil {
		sourceHeight = media.Video.Height
		sourceFrameRate = media.Video.FrameRate
		sourceVideoCodec = media.Video.Codec
		sourceColorTransfer = media.Video.ColorTransfer
	}
	var args []string
	var encoderName, encoderKind, hardwareDecoder string
	toneMap := false
	sessionKind := SessionRemux
	if decision.Mode == stream.ModeUnsupported || requested || ForcedTranscode(r) {
		s.deriveTranscodeCeiling(r.Context())
		encoder, available := s.binary.Capabilities(r.Context()).Best()
		if !available {
			return &DeliveryError{Status: http.StatusUnsupportedMediaType, Code: "video_transcode_required"}
		}
		s.transcodes.setKind(encoder.Kind)
		release, available := s.transcodes.acquire(stream.ToneMap(sourceColorTransfer))
		if !available {
			return &DeliveryError{Status: http.StatusServiceUnavailable, Code: "transcode_busy", RetryAfter: 1}
		}
		defer release()
		decision.Mode = stream.ModeTranscode
		sessionKind = SessionTranscode
		encoderName = encoder.Name
		encoderKind = string(encoder.Kind)
		hardwareDecoder = s.binary.HardwareDecoder(r.Context())
		toneMap = stream.ToneMap(sourceColorTransfer)
		args = stream.TranscodeArgs(path, decision, start, stream.TranscodeOptions{
			Encoder: encoder.Name, HWAccel: hardwareDecoder, Height: height,
			SourceHeight: sourceHeight, ColorTransfer: sourceColorTransfer, AudioStreamIndex: audioIndexOf(selected),
		})
	} else if index := audioIndexOf(selected); index != nil {
		args = stream.RemuxArgsForAudio(path, decision, start, *index)
	} else {
		args = stream.RemuxArgs(path, decision, start)
	}
	s.log.Info("starting media stream",
		"mode", decision.Mode,
		"reason", decision.Reason,
		"start_seconds", start,
		"requested_height", height,
		"source_height", sourceHeight,
		"source_video_codec", sourceVideoCodec,
		"source_frame_rate", sourceFrameRate,
		"source_color_transfer", sourceColorTransfer,
		"audio_action", decision.Audio,
		"encoder", encoderName,
		"encoder_kind", encoderKind,
		"hardware_decoder", hardwareDecoder,
		"tone_map", toneMap,
	)
	// Reserved before the process exists, so a refusal costs no spawn: a slot
	// refused here is the remux ceiling, answered with the one code the
	// interface already retries.
	slot := s.sessions.Acquire(sessionKind, path)
	if slot == nil {
		return &DeliveryError{Status: http.StatusServiceUnavailable, Code: "transcode_busy", RetryAfter: 1}
	}
	defer slot.Release()
	streamStarted := time.Now()
	cmd := exec.CommandContext(r.Context(), binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &DeliveryError{Status: http.StatusInternalServerError, Code: "stream_start_failed"}
	}
	stderr := boundedio.NewTail(64 << 10)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return &DeliveryError{Status: http.StatusInternalServerError, Code: "stream_start_failed"}
	}
	// Attached before the first byte is read, so a stream is findable at
	// shutdown from the moment a process exists, not from the moment it
	// produces output.
	slot.Attach(cmd.Process)
	// Do not send a successful video response for an encoder that never opens.
	reader := bufio.NewReader(stdout)
	if _, err := reader.Peek(1); err != nil {
		_ = cmd.Wait()
		s.log.Warn("media encoder failed before output", "detail", diagnosticTail(stderr.String(), 4096))
		return &DeliveryError{Status: http.StatusBadGateway, Code: "stream_encode_failed"}
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Accept-Ranges", "none")
	w.WriteHeader(http.StatusOK)
	// Push the headers out before the first copy. The compression middleware
	// holds a response's first kilobyte and a half back while it decides, and
	// the status with them; one flush here commits the encoding decision and
	// lets the first bytes follow as soon as the encoder produces them.
	// Measured on this exact spot against the same machine and the same film,
	// ten runs each: without the flush the first byte took about 75 ms on a
	// plain client and 78 ms through the compression wrapper; with it, 48 ms
	// and 60 ms. Kept because it helps, measured before it was kept.
	if err := http.NewResponseController(w).Flush(); err != nil {
		s.log.Debug("flushing the response start was not supported", "error", err)
	}
	bytesWritten, copyErr := io.Copy(w, reader)
	if copyErr != nil {
		_ = cmd.Process.Kill()
	}
	processErr := cmd.Wait()
	s.log.Debug("media stream ended",
		"mode", decision.Mode,
		"duration", time.Since(streamStarted),
		"bytes_written", bytesWritten,
		"copy_error", errorText(copyErr),
		"process_error", errorText(processErr),
		"canceled", r.Context().Err() != nil,
	)
	if processErr != nil && r.Context().Err() == nil && copyErr == nil {
		s.log.Warn("media encoder ended early", "error", processErr, "detail", diagnosticTail(stderr.String(), 4096))
	}
	return nil
}

// KillStreams terminates every live converted stream.
//
// main calls it on every path that ends the process: before draining the HTTP
// server, where killing first also unblocks the handlers that would otherwise
// hold the drain open for the length of a film, and before every os.Exit,
// which skips deferred calls by definition. A killed encoder loses its stdout
// pipe, the copy in its handler returns, and the process is reaped by the
// handler's own Wait.
func (s *Service) KillStreams() {
	if killed := s.sessions.KillAll(); killed > 0 {
		s.log.Info("killed live media streams for shutdown", "count", killed)
	}
}

// ActiveStreams reports how many converted streams are live. Diagnostics and
// tests read it; no request path does.
func (s *Service) ActiveStreams() int {
	return s.sessions.Active()
}

// TranscodeStatus reports what the interface's quality menu needs: the best
// encoder this machine has, and whether every transcode slot is taken.
// Calling it without a binary on disk is the caller's mistake to gate -- the
// underlying probe would otherwise download one, and asking a question must
// never do that.
func (s *Service) TranscodeStatus(ctx context.Context) (kind, encoder string, busy bool, ok bool) {
	s.deriveTranscodeCeiling(ctx)
	best, available := s.binary.Capabilities(ctx).Best()
	if !available {
		return "", "", false, false
	}
	return string(best.Kind), best.Name, s.transcodes.busy(), true
}

// deriveTranscodeCeiling sizes the transcode budget from the encoder probe,
// once per process. It refuses to cause the probe's own download: a machine
// with no binary on disk keeps the software ceiling of one, which is the
// budget that matters until something has actually been encoded.
func (s *Service) deriveTranscodeCeiling(ctx context.Context) {
	s.transcodeCeilingOnce.Do(func() {
		if s.binary == nil || !s.binary.Available() {
			return
		}
		if best, ok := s.binary.Capabilities(ctx).Best(); ok {
			s.transcodes.setKind(best.Kind)
		}
	})
}

// RequestedHeight reads ?h=. Absent means "leave the picture alone".
func RequestedHeight(r *http.Request) (int, bool, *DeliveryError) {
	raw, present := r.URL.Query()["h"]
	if !present {
		return 0, false, nil
	}
	if len(raw) != 1 {
		return 0, false, &DeliveryError{Status: http.StatusBadRequest, Code: "invalid_height"}
	}
	height, err := strconv.Atoi(raw[0])
	if err != nil || height <= 0 {
		return 0, false, &DeliveryError{Status: http.StatusBadRequest, Code: "invalid_height"}
	}
	// Only a rung this server actually offers. An arbitrary number would let a
	// caller ask for a scale nobody sized a bitrate for.
	for _, offered := range stream.Qualities {
		if offered == height {
			return height, true, nil
		}
	}
	return 0, false, &DeliveryError{Status: http.StatusBadRequest, Code: "invalid_height"}
}

// RequestedStart reads ?t=, the second the stream should begin at. Absent
// means the beginning.
func RequestedStart(r *http.Request) (float64, *DeliveryError) {
	value := r.URL.Query().Get("t")
	if value == "" {
		return 0, nil
	}
	start, err := strconv.ParseFloat(value, 64)
	if err != nil || start < 0 || math.IsNaN(start) || math.IsInf(start, 0) {
		return 0, &DeliveryError{Status: http.StatusBadRequest, Code: "invalid_position"}
	}
	return start, nil
}

// ForcedTranscode reads ?video=transcode.
//
// This is how a browser reports what no server can know. `hevc` is classified
// risky rather than unsupported because Safari plays it and Chrome does not, so
// the remux is worth attempting -- and when it fails it fails silently, with
// sound over a picture that never arrives. The player detects that (videoWidth
// stays 0) and asks again with this flag, which turns the one dead end M1 could
// not resolve into a film that plays.
func ForcedTranscode(r *http.Request) bool {
	return r.URL.Query().Get("video") == "transcode"
}

// audioIndexOf maps the selected track onto the stream index the arguments
// need, keeping the pointer-vs-value choice out of the branches above.
func audioIndexOf(selected *library.AudioTrack) *int {
	if selected == nil {
		return nil
	}
	value := selected.StreamIndex
	return &value
}

func diagnosticTail(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maximum {
		return value
	}
	return "[truncated] " + value[len(value)-maximum:]
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
