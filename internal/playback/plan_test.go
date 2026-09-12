package playback

import (
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

func measured(path, video, audio string) library.FileMedia {
	media := library.FileMedia{Status: library.MediaOK, Container: "test"}
	if video != "" {
		media.Video = &library.VideoStream{StreamIndex: 0, Codec: video, Width: 1920, Height: 1080}
	}
	if audio != "" {
		media.AudioTracks = []library.AudioTrack{{StreamIndex: 1, Codec: audio, IsDefault: true}}
	}
	return media
}

func TestInfoDecisionMatchesTheOldHandlerPolicy(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		media         library.FileMedia
		audioSelected bool
		canTranscode  bool
		wantMode      stream.Mode
		wantReason    string
	}{
		{
			name:       "unmeasured mp4 stays direct",
			path:       "film.mp4",
			wantMode:   stream.ModeDirect,
			wantReason: "container_direct",
		},
		{
			name:       "unmeasured mkv stays remux",
			path:       "film.mkv",
			wantMode:   stream.ModeRemux,
			wantReason: "container_remux",
		},
		{
			name:       "measured friendly file plays as it is",
			path:       "film.mp4",
			media:      measured("film.mp4", "h264", "aac"),
			wantMode:   stream.ModeDirect,
			wantReason: "direct_play",
		},
		{
			name:       "measured ac3 mkv remuxes its audio",
			path:       "film.mkv",
			media:      measured("film.mkv", "h264", "ac3"),
			wantMode:   stream.ModeRemux,
			wantReason: "audio_transcode",
		},
		{
			name:       "unsupported without an encoder is a refusal",
			path:       "film.mp4",
			media:      measured("film.mp4", "mpeg2video", "mp2"),
			wantMode:   stream.ModeUnsupported,
			wantReason: "video_transcode_required",
		},
		{
			name:         "unsupported with an encoder becomes a transcode",
			path:         "film.mp4",
			media:        measured("film.mp4", "mpeg2video", "mp2"),
			canTranscode: true,
			wantMode:     stream.ModeTranscode,
			wantReason:   "video_transcode",
		},
		{
			name:          "a selected track forces the remux route",
			path:          "film.mp4",
			media:         measured("film.mp4", "h264", "aac"),
			audioSelected: true,
			wantMode:      stream.ModeRemux,
			wantReason:    "audio_track_selected",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var selected *library.AudioTrack
			if tc.audioSelected {
				track := tc.media.AudioTracks[0]
				selected = &track
			}
			decision, reason := InfoDecision(tc.path, tc.media, selected, tc.audioSelected, tc.canTranscode)
			if decision.Mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", decision.Mode, tc.wantMode)
			}
			if reason != tc.wantReason {
				t.Errorf("reason code = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

// TestStreamDecisionDefaultsTheBrowserReadableTrack pins the compatibility
// choice: with no track asked for, the one the browser can decode is mapped
// explicitly, so a DTS-first BluRay does not burn a core transcoding past an
// AAC track sitting one index away.
func TestStreamDecisionDefaultsTheBrowserReadableTrack(t *testing.T) {
	media := library.FileMedia{
		Status: library.MediaOK,
		Video:  &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []library.AudioTrack{
			{StreamIndex: 1, Codec: "dts", IsDefault: true},
			{StreamIndex: 2, Codec: "aac"},
		},
	}
	decision, selected := StreamDecision("film.mkv", media, nil, false)
	if decision.Audio != stream.AudioCopy {
		t.Errorf("audio action = %q, want copy: the mapped track is the browser-readable one", decision.Audio)
	}
	if selected == nil || selected.Codec != "aac" {
		t.Errorf("mapped track = %+v, want the aac one over the file's dts default", selected)
	}

	asked := media.AudioTracks[0]
	decision, selected = StreamDecision("film.mkv", media, &asked, true)
	if selected != &asked {
		t.Errorf("explicit selection = %+v, want the asked track untouched", selected)
	}
	if decision.Mode != stream.ModeRemux {
		t.Errorf("mode = %q, want remux", decision.Mode)
	}
}

// TestStreamDecisionToleratesUnmeasuredMedia keeps the planner total: a nil
// video yields the unsupported answer instead of a panic, whatever calls it.
func TestStreamDecisionToleratesUnmeasuredMedia(t *testing.T) {
	decision, _ := StreamDecision("film.mp4", library.FileMedia{}, nil, false)
	if decision.Mode != stream.ModeUnsupported {
		t.Errorf("mode = %q, want unsupported", decision.Mode)
	}
}
