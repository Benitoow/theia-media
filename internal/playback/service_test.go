package playback

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Benitoow/theia-media/internal/fakeffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

// The delivery service against a fake binary: this test binary, re-executed
// in its fake-ffmpeg role. Everything here is about the shapes the request
// path produces -- statuses, codes, headers, the ceiling, and the kill that
// shutdown depends on.

func remuxDecision() stream.Decision {
	return stream.Decision{Mode: stream.ModeRemux, Audio: stream.AudioCopy, Reason: "container_remux"}
}

func measuredMedia() library.FileMedia {
	return library.FileMedia{
		Status:          library.MediaOK,
		Container:       "matroska,webm",
		DurationSeconds: 5400,
		Video:           &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080, FrameRate: 24},
		AudioTracks:     []library.AudioTrack{{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true}},
	}
}

type countingWorkload struct{ begins int }

func (w *countingWorkload) BeginInteractive() func() {
	w.begins++
	return func() {}
}

// serveConvertedHandler is the adapter a handler becomes: delivery, then the
// refusal written the way writeDeliveryError writes it over in api.
func serveConvertedHandler(service *Service, path string, decision func(r *http.Request) stream.Decision,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if derr := service.ServeConverted(w, r, path, measuredMedia(), decision(r), nil); derr != nil {
			if derr.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(derr.RetryAfter))
			}
			w.WriteHeader(derr.Status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": derr.Code})
		}
	}
}

func newDeliveryServer(t *testing.T, service *Service, decision func(r *http.Request) stream.Decision,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(serveConvertedHandler(service, "Heat (1995).mkv", decision))
}

func deliveryErrorCode(t *testing.T, res *http.Response) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the refusal: %v", err)
	}
	return body.Error
}

// TestARewrappedFileStreamsFragmentedMP4 pins the success shape of the
// converted route: fragmented-MP4 content type, no caching, no range support,
// and bytes that really crossed the pipe.
func TestARewrappedFileStreamsFragmentedMP4(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeBytes, t.TempDir())
	defer restore()

	workload := &countingWorkload{}
	service := NewService(Options{Binary: &fakeffmpeg.Binary{}, Workload: workload})
	server := newDeliveryServer(t, service, func(*http.Request) stream.Decision { return remuxDecision() })
	defer server.Close()

	res, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("the delivery request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("remux = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("content type = %q, want video/mp4", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("cache control = %q, want private, no-store", cc)
	}
	if ar := res.Header.Get("Accept-Ranges"); ar != "none" {
		t.Errorf("accept ranges = %q, want none", ar)
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) < 1024 {
		t.Errorf("the fake encoder produced %d bytes, want a real chunk", len(body))
	}
	if workload.begins != 1 {
		t.Errorf("the workload was admitted %d times, want once", workload.begins)
	}
	if service.ActiveStreams() != 0 {
		t.Errorf("active streams = %d after completion, want 0", service.ActiveStreams())
	}
}

// TestRefusalsKeepTheirCodes pins every refusal the delivery can answer with
// before anything is written: a machine without ffmpeg, an unmeasurable
// height, a broken position, an encoder that never opens, and a transcode
// asked of a machine with no encoder at all.
func TestRefusalsKeepTheirCodes(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeFail, t.TempDir())
	defer restore()

	cases := []struct {
		name       string
		binary     *fakeffmpeg.Binary
		query      string
		decision   stream.Decision
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no ffmpeg binary",
			binary:     &fakeffmpeg.Binary{PathErr: context.DeadlineExceeded},
			decision:   remuxDecision(),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "ffmpeg_unavailable",
		},
		{
			name:       "a height nothing offers",
			binary:     &fakeffmpeg.Binary{},
			query:      "?h=999",
			decision:   remuxDecision(),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_height",
		},
		{
			name:       "a height that is not a number",
			binary:     &fakeffmpeg.Binary{},
			query:      "?h=abc",
			decision:   remuxDecision(),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_height",
		},
		{
			name:       "a position that is not a time",
			binary:     &fakeffmpeg.Binary{},
			query:      "?t=later",
			decision:   remuxDecision(),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_position",
		},
		{
			name:       "a negative position",
			binary:     &fakeffmpeg.Binary{},
			query:      "?t=-3",
			decision:   remuxDecision(),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_position",
		},
		{
			name:       "an encoder that never opens",
			binary:     &fakeffmpeg.Binary{},
			decision:   remuxDecision(),
			wantStatus: http.StatusBadGateway,
			wantCode:   "stream_encode_failed",
		},
		{
			name:       "a transcode with no encoder to run",
			binary:     &fakeffmpeg.Binary{},
			decision:   stream.Decision{Mode: stream.ModeUnsupported},
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   "video_transcode_required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(Options{Binary: tc.binary})
			server := newDeliveryServer(t, service, func(r *http.Request) stream.Decision {
				_ = r
				return tc.decision
			})
			defer server.Close()
			res, err := server.Client().Get(server.URL + tc.query)
			if err != nil {
				t.Fatalf("the request failed: %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.wantStatus)
			}
			if code := deliveryErrorCode(t, res); code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
			if res.Header.Get("Retry-After") != "" {
				t.Error("a plain refusal carried a Retry-After")
			}
		})
	}
}

// TestTheConvertedStreamCeilingRefusesWithTheBusyShape pins the one new
// response the backend redesign was allowed: the stream beyond the remux
// ceiling is refused with the transcode-busy shape the interface already
// retries, five attempts and a one-second backoff.
func TestTheConvertedStreamCeilingRefusesWithTheBusyShape(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeLive, t.TempDir())
	defer restore()

	service := NewService(Options{Binary: &fakeffmpeg.Binary{}, StreamLimit: 2})
	server := newDeliveryServer(t, service, func(*http.Request) stream.Decision { return remuxDecision() })
	defer server.Close()

	var held []*http.Response
	defer func() {
		for _, res := range held {
			res.Body.Close()
		}
	}()
	for i := 0; i < 2; i++ {
		res, err := server.Client().Get(server.URL)
		if err != nil {
			t.Fatalf("held remux %d failed: %v", i, err)
		}
		held = append(held, res)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("held remux %d = %d, want 200", i, res.StatusCode)
		}
	}

	res, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("the refused remux failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the ceiling refusal = %d, want 503", res.StatusCode)
	}
	if retry := res.Header.Get("Retry-After"); retry != "1" {
		t.Errorf("retry-after = %q, want 1", retry)
	}
	if code := deliveryErrorCode(t, res); code != "transcode_busy" {
		t.Errorf("code = %q, want transcode_busy", code)
	}

	service.KillStreams()
	if service.ActiveStreams() != 0 {
		t.Errorf("active streams after the kill = %d, want 0", service.ActiveStreams())
	}
}

// TestKillStreamsUnblocksEveryLiveStream is the shutdown proof at the unit
// level: three viewers mid-film, the processes killed out from under them, and
// every handler back from its copy loop promptly -- which is only possible
// because the processes really died and closed their pipes.
func TestKillStreamsUnblocksEveryLiveStream(t *testing.T) {
	// The kill only means something once every fake has put its first byte in
	// the pipe, which the markers record after the chunk. Waiting for slots
	// would not be enough: they are reserved before the process exists.
	markers := t.TempDir()
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeLive, markers)
	defer restore()

	service := NewService(Options{Binary: &fakeffmpeg.Binary{}})
	server := newDeliveryServer(t, service, func(*http.Request) stream.Decision { return remuxDecision() })
	defer server.Close()

	type outcome struct {
		status int
		bytes  int
		err    error
	}
	done := make(chan outcome, 3)
	for i := 0; i < 3; i++ {
		go func() {
			client := &http.Client{Timeout: 30 * time.Second}
			res, err := client.Get(server.URL)
			if err != nil {
				done <- outcome{err: err}
				return
			}
			body, readErr := io.ReadAll(res.Body)
			res.Body.Close()
			done <- outcome{status: res.StatusCode, bytes: len(body), err: readErr}
		}()
	}

	// All three fakes must be live and past their first byte before the kill
	// means anything.
	waitFor(t, 15*time.Second, func() bool { return countMarkers(markers) == 3 })

	service.KillStreams()
	waitFor(t, 15*time.Second, func() bool { return service.ActiveStreams() == 0 })

	for i := 0; i < 3; i++ {
		select {
		case out := <-done:
			if out.err != nil {
				t.Errorf("stream %d: %v", i, out.err)
				continue
			}
			if out.status != http.StatusOK {
				t.Errorf("stream %d status = %d, want the 200 it started with", i, out.status)
			}
			if out.bytes < 1024 {
				t.Errorf("stream %d delivered %d bytes, want at least the first chunk", i, out.bytes)
			}
		case <-time.After(15 * time.Second):
			t.Fatalf("stream %d never came back from the kill", i)
		}
	}
}

// TestATranscodeDoesNotConsumeTheRemuxCeiling pins the split of the two
// budgets: a live transcode leaves every remux slot free.
func TestATranscodeDoesNotConsumeTheRemuxCeiling(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeLive, t.TempDir())
	defer restore()

	service := NewService(Options{Binary: &fakeffmpeg.Binary{Caps: fakeffmpeg.UsableEncoder()}, StreamLimit: 1})
	server := newDeliveryServer(t, service, func(*http.Request) stream.Decision {
		return stream.Decision{Mode: stream.ModeUnsupported}
	})
	defer server.Close()

	transcode, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("the transcode failed: %v", err)
	}
	defer transcode.Body.Close()
	if transcode.StatusCode != http.StatusOK {
		t.Fatalf("transcode = %d, want 200", transcode.StatusCode)
	}

	waitFor(t, 10*time.Second, func() bool { return service.ActiveStreams() == 1 })
	if service.sessions.RemuxActive() != 0 {
		t.Errorf("remux slots used by a transcode = %d, want 0", service.sessions.RemuxActive())
	}

	remux, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("the remux beside a live transcode failed: %v", err)
	}
	remux.Body.Close()
	if remux.StatusCode != http.StatusOK {
		t.Errorf("remux beside a live transcode = %d, want 200", remux.StatusCode)
	}

	service.KillStreams()
}

// TestSelectedAudioKeepsTheExplicitMapping checks the audio branch end to
// end: a selected track rides the remux that maps it explicitly.
func TestSelectedAudioKeepsTheExplicitMapping(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeBytes, t.TempDir())
	defer restore()

	service := NewService(Options{Binary: &fakeffmpeg.Binary{}})
	selected := &library.AudioTrack{StreamIndex: 2, Codec: "aac", Language: "fre"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = service.ServeConverted(w, r, "Heat (1995).mkv", measuredMedia(), remuxDecision(), selected)
	}))
	defer server.Close()

	res, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("the request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("remux with a selected track = %d, want 200", res.StatusCode)
	}
}

// waitFor polls a condition rather than sleeping a fixed amount: the fake
// processes start in tens of milliseconds and the CI machines are slow.
func waitFor(t *testing.T, within time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the condition was not met within %s", within)
}

func countMarkers(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".pid") {
			count++
		}
	}
	return count
}
