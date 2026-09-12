// Package playback owns the delivery policy shared by every playback route.
//
// Before the backend redesign (docs/plan-refonte-lecture.md, tranche 2), the
// decision of how a file reaches the browser was computed four times across
// the film and episode handlers, and the /info answer could drift from the
// stream that followed it. The functions here are the single source: /info and
// the remux route both ask, with different inputs, and can no longer disagree.
//
// The package is deliberately small. Identity resolution and persistence stay
// in the handlers -- films and episodes keep separate tables and route
// families (decision 39); what they share is the policy, and that lives here.
package playback

import (
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/stream"
)

// InfoDecision is the decision /info reports.
//
// The answer starts from the container alone (the M1 promise: asking how a
// file will play must never run, let alone download, ffmpeg), upgrades to the
// full decision once the file has been measured, forces a remux when a
// specific audio track was asked for, and finally rewrites an unsupported
// codec into a transcode when this machine has an encoder that runs
// (V2-M6, decision 59). The reason code travels with it, because /info never
// sends English sentences: the interface owns every wording (decision 25).
func InfoDecision(path string, media library.FileMedia, selected *library.AudioTrack,
	audioRequested, canTranscode bool,
) (stream.Decision, string) {
	decision := stream.DecideByContainer(path)
	reasonCode := "container_remux"
	if decision.Mode == stream.ModeDirect {
		reasonCode = "container_direct"
	}
	if media.Status == library.MediaOK && media.Video != nil {
		decision = stream.Decide(path, media.Video.Codec, audioCodecOf(media, selected))
		reasonCode = ReasonCode(decision)
	}
	if audioRequested && decision.Mode == stream.ModeDirect {
		// Direct play hands the whole file to the browser and cannot guarantee
		// a chosen track. A manual selection therefore takes the remux route
		// even when the MP4 itself would otherwise play untouched.
		decision.Mode = stream.ModeRemux
		decision.Audio = stream.AudioCopy
		decision.Reason = "a selected audio track must be mapped explicitly"
		reasonCode = "audio_track_selected"
	}
	if decision.Mode == stream.ModeUnsupported && canTranscode {
		decision.Mode = stream.ModeTranscode
		reasonCode = "video_transcode"
	}
	return decision, reasonCode
}

// StreamDecision is the decision the remux route acts on, once the media has
// been measured or refused. When no track was asked for, the one the browser
// can decode is mapped explicitly -- not a silent quality decision, but the
// compatibility pick that keeps a DTS-first BluRay from burning a core
// transcoding past an AAC track sitting one index away.
func StreamDecision(path string, media library.FileMedia, selected *library.AudioTrack,
	audioRequested bool,
) (stream.Decision, *library.AudioTrack) {
	if selected == nil {
		if track, ok := stream.PreferredAudio(media.AudioTracks, codecOfTrack); ok {
			defaulted := track
			selected = &defaulted
		}
	}
	decision := stream.Decide(path, videoCodecOf(media), audioCodecOf(media, selected))
	if audioRequested && decision.Mode == stream.ModeDirect {
		decision.Mode = stream.ModeRemux
	}
	return decision, selected
}

// ReasonCode names the decision for the interface. The stream package's own
// reason strings are English sentences for logs; this is the stable code the
// browser stores its verdict under.
func ReasonCode(decision stream.Decision) string {
	switch {
	case decision.Mode == stream.ModeUnsupported:
		return "video_transcode_required"
	case decision.Mode == stream.ModeDirect:
		return "direct_play"
	case decision.Audio == stream.AudioTranscode:
		return "audio_transcode"
	default:
		return "container_remux"
	}
}

// videoCodecOf is the measured codec as recorded, or empty when the file has
// not been measured. The stream package lowercases internally, so the raw
// value travels. A planner precondition makes nil video unreachable for the
// stream route, which only runs after a successful probe; the guard keeps the
// planner itself total.
func videoCodecOf(media library.FileMedia) string {
	if media.Status != library.MediaOK || media.Video == nil {
		return ""
	}
	return media.Video.Codec
}

func audioCodecOf(media library.FileMedia, selected *library.AudioTrack) string {
	if selected != nil {
		return selected.Codec
	}
	if track, ok := stream.PreferredAudio(media.AudioTracks, codecOfTrack); ok {
		return track.Codec
	}
	return ""
}

func codecOfTrack(track library.AudioTrack) string { return track.Codec }
