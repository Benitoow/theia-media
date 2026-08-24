package stream

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDecideByContainer(t *testing.T) {
	// This is the decision made without running anything, and it is what keeps
	// the promise that ffmpeg is only downloaded when something actually needs
	// it. A library of MP4s must never reach the remux branch.
	tests := []struct {
		path string
		want Mode
	}{
		{"/films/Film.mp4", ModeDirect},
		{"/films/Film.M4V", ModeDirect},
		{"/films/Film.webm", ModeDirect},
		{"/films/Film.mkv", ModeRemux},
		{"/films/Film.avi", ModeRemux},
		{"/films/Film.ts", ModeRemux},
	}
	for _, tt := range tests {
		if got := DecideByContainer(tt.path); got.Mode != tt.want {
			t.Errorf("DecideByContainer(%q) = %q, want %q", tt.path, got.Mode, tt.want)
		}
	}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		video     string
		audio     string
		wantMode  Mode
		wantAudio AudioAction
	}{
		{
			name:     "an ordinary MP4 needs nothing",
			path:     "f.mp4",
			video:    "h264",
			audio:    "aac",
			wantMode: ModeDirect,
		},
		{
			name:     "a silent H.264 MP4 still plays directly",
			path:     "f.mp4",
			video:    "h264",
			audio:    "",
			wantMode: ModeDirect,
		},
		{
			// The case decision 1 exists for. Remuxing without touching the
			// audio would produce a film with a picture and no sound.
			name:      "H.264 and AC3 in a MKV remuxes and re-encodes the audio",
			path:      "f.mkv",
			video:     "h264",
			audio:     "ac3",
			wantMode:  ModeRemux,
			wantAudio: AudioTranscode,
		},
		{
			name:      "DTS is re-encoded too",
			path:      "f.mkv",
			video:     "h264",
			audio:     "dts",
			wantMode:  ModeRemux,
			wantAudio: AudioTranscode,
		},
		{
			name:      "TrueHD is re-encoded too",
			path:      "f.mkv",
			video:     "h264",
			audio:     "truehd",
			wantMode:  ModeRemux,
			wantAudio: AudioTranscode,
		},
		{
			name:      "friendly streams in an unfriendly container are copied",
			path:      "f.mkv",
			video:     "h264",
			audio:     "aac",
			wantMode:  ModeRemux,
			wantAudio: AudioCopy,
		},
		{
			// The picture needs an encoder that runs here (decision 58). The
			// sound still needs what it always needed, and this decision is
			// what a transcode is built from.
			name:      "MPEG-2 needs a full transcode and still names its audio",
			path:      "f.ts",
			video:     "mpeg2video",
			audio:     "ac3",
			wantMode:  ModeUnsupported,
			wantAudio: AudioTranscode,
		},
		{
			name:      "VC-1 needs a full transcode and still names its audio",
			path:      "f.mkv",
			video:     "vc1",
			audio:     "ac3",
			wantMode:  ModeUnsupported,
			wantAudio: AudioTranscode,
		},
		{
			name:      "an unsupported picture over friendly sound copies the sound",
			path:      "f.mkv",
			video:     "vc1",
			audio:     "aac",
			wantMode:  ModeUnsupported,
			wantAudio: AudioCopy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.path, tt.video, tt.audio)
			if got.Mode != tt.wantMode {
				t.Fatalf("mode = %q, want %q (%s)", got.Mode, tt.wantMode, got.Reason)
			}
			if tt.wantAudio != "" && got.Audio != tt.wantAudio {
				t.Errorf("audio = %q, want %q", got.Audio, tt.wantAudio)
			}
		})
	}
}

func TestDecideFlagsHEVCAsRisky(t *testing.T) {
	// HEVC copies into MP4 and plays in Safari but generally not in Chrome. The
	// remux is worth attempting; the flag lets the interface warn instead of
	// looking broken.
	got := Decide("f.mkv", "hevc", "aac")
	if got.Mode != ModeRemux {
		t.Fatalf("mode = %q, want remux", got.Mode)
	}
	if !got.VideoRisky {
		t.Error("HEVC should be flagged as risky")
	}
}

func TestRemuxArgsCopyVideoAndTranscodeAudio(t *testing.T) {
	args := RemuxArgs("/films/f.mkv", Decision{Mode: ModeRemux, Audio: AudioTranscode}, 0)

	// Video is always copied: v1 does not re-encode pictures.
	if i := slices.Index(args, "-c:v"); i < 0 || args[i+1] != "copy" {
		t.Errorf("args = %v, want the video copied", args)
	}
	if i := slices.Index(args, "-c:a"); i < 0 || args[i+1] != "aac" {
		t.Errorf("args = %v, want the audio re-encoded to AAC", args)
	}
	// A plain MP4 has to rewind to write its index, which a pipe cannot do.
	if i := slices.Index(args, "-movflags"); i < 0 || args[i+1] != "+frag_keyframe+empty_moov+default_base_moof" {
		t.Errorf("args = %v, want a fragmented MP4", args)
	}
	if args[len(args)-1] != "pipe:1" {
		t.Errorf("args = %v, want output on stdout", args)
	}
	if slices.Contains(args, "-ss") {
		t.Errorf("args = %v, want no seek when starting at zero", args)
	}
}

func TestRemuxArgsSeekBeforeInput(t *testing.T) {
	args := RemuxArgs("/films/f.mkv", Decision{Mode: ModeRemux, Audio: AudioCopy}, 90)

	ss := slices.Index(args, "-ss")
	in := slices.Index(args, "-i")
	if ss < 0 {
		t.Fatalf("args = %v, want a seek", args)
	}
	// -ss before -i seeks by keyframe without decoding everything before it,
	// which is the difference between instant and unusable on a long film.
	if ss > in {
		t.Errorf("args = %v, want -ss before -i", args)
	}
	if args[ss+1] != "90.000" {
		t.Errorf("seek = %q, want 90.000", args[ss+1])
	}
}

func TestRemuxArgsForAudioMapsOnlyTheSelectedAbsoluteStream(t *testing.T) {
	args := RemuxArgsForAudio("/films/f.mkv",
		Decision{Mode: ModeRemux, Audio: AudioTranscode}, 0, 4)

	indices := make([]int, 0, 2)
	for i, arg := range args {
		if arg == "-map" {
			indices = append(indices, i)
		}
	}
	if len(indices) != 2 {
		t.Fatalf("args = %v, want two map arguments", args)
	}
	if got := args[indices[0]+1]; got != "0:v:0" {
		t.Errorf("video map = %q, want 0:v:0", got)
	}
	if got := args[indices[1]+1]; got != "0:4" {
		t.Errorf("audio map = %q, want selected absolute stream 0:4", got)
	}
}

// The CPU-only encoding path. Hardware encoders carry their own defaults; the
// software one is where the arguments actually decide whether a modest machine
// keeps up with playback.
func TestTheSoftwareEncoderIsNotTunedForAVideoCall(t *testing.T) {
	args := TranscodeArgs("/films/heat.mkv", Decision{Audio: AudioTranscode}, 0, "libx264", "", 720, nil)

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "zerolatency") {
		t.Error("libx264 is still tuned zerolatency: it costs 18% throughput, b-frames and lookahead " +
			"to save an encoder delay a buffered player cannot perceive")
	}
	if !strings.Contains(joined, "-preset veryfast") {
		t.Error("libx264 lost its veryfast preset, which is what keeps it ahead of playback")
	}
	if !strings.Contains(joined, "-threads ") {
		t.Error("libx264 has no thread cap, which is what keeps the first fragment quick")
	}
}

func TestTheThreadCapNeverAsksForMoreThanTheMachineHas(t *testing.T) {
	got := x264Threads()
	if got < 1 {
		t.Fatalf("x264Threads() = %d, want at least one", got)
	}
	if got > 8 {
		t.Errorf("x264Threads() = %d, want no more than 8: past that x264 buys latency, not speed", got)
	}
	if cpus := runtime.NumCPU(); got > cpus {
		t.Errorf("x264Threads() = %d on a %d-cpu machine", got, cpus)
	}
}

// A hardware encoder must not pick up the software encoder's arguments: its own
// defaults are what the vendor tuned, and -preset means something different or
// nothing at all to each of them.
func TestAHardwareEncoderKeepsItsOwnSettings(t *testing.T) {
	for _, encoder := range []string{"h264_amf", "h264_nvenc", "h264_qsv", "h264_mf"} {
		joined := strings.Join(
			TranscodeArgs("/films/heat.mkv", Decision{}, 0, encoder, "", 720, nil), " ")
		if strings.Contains(joined, "-preset") || strings.Contains(joined, "-threads") {
			t.Errorf("%s was given libx264's arguments: %s", encoder, joined)
		}
	}
}

// The regression that this file exists to hold down.
//
// A decision whose mode is unsupported is the input to a transcode, not a dead
// end, since decision 58. It once arrived there with no audio action at all, so
// TranscodeArgs took the copy branch and wrote AC3 into an MP4: a film with a
// picture and no sound, which is precisely what decision 1 was written to
// prevent on the remux path.
func TestATranscodedFilmKeepsItsSound(t *testing.T) {
	for _, video := range []string{"mpeg2video", "vc1"} {
		decision := Decide("/films/dvd.mkv", video, "ac3")
		joined := strings.Join(
			TranscodeArgs("/films/dvd.mkv", decision, 0, "libx264", "", 720, nil), " ")
		if strings.Contains(joined, "-c:a copy") {
			t.Errorf("%s + AC3 copies its audio into MP4, which is a silent film: %s",
				video, joined)
		}
		if !strings.Contains(joined, "-c:a aac") {
			t.Errorf("%s + AC3 does not re-encode its audio: %s", video, joined)
		}
	}
}
