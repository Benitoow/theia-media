package stream

import (
	"slices"
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
			// Out of scope by decision 1: re-encoding video is the expensive
			// one, and saying so beats pinning a CPU for two hours.
			name:     "MPEG-2 would need a full transcode",
			path:     "f.ts",
			video:    "mpeg2video",
			audio:    "ac3",
			wantMode: ModeUnsupported,
		},
		{
			name:     "VC-1 would need a full transcode",
			path:     "f.mkv",
			video:    "vc1",
			audio:    "ac3",
			wantMode: ModeUnsupported,
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
