// Package stream decides how a file should reach the browser, and produces the
// bytes.
//
// Two paths, deliberately unequal in cost. Direct play hands the file over
// untouched with range requests, which is what should happen for most of a
// modern library and needs no ffmpeg at all. Remuxing rewraps the streams into
// a fragmented MP4 on the fly, re-encoding the audio when the browser cannot
// decode it.
//
// Full video transcoding is out of scope for v1 (decision 1 in
// docs/DECISIONS.md). That is the expensive one, and a file whose video codec
// no browser reads is reported as such rather than quietly eating a CPU.
package stream

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Mode is how a file will be delivered.
type Mode string

const (
	// ModeDirect sends the file as-is. Seeking works, nothing is spawned.
	ModeDirect Mode = "direct"

	// ModeRemux rewraps into fragmented MP4, copying the video and copying or
	// re-encoding the audio.
	ModeRemux Mode = "remux"

	// ModeUnsupported means the video codec would need a full transcode and
	// this machine has no encoder that runs.
	ModeUnsupported Mode = "unsupported"

	// ModeTranscode re-encodes the picture. V2-M6, and the thing decision 1
	// deliberately kept out of v1 -- "the real furnace".
	//
	// What changed is not the appetite for it but the evidence. The spec put
	// GPU transcoding in v2 "only if direct play and CPU remux prove
	// insufficient in real use", and they did: an HEVC rip the maintainer
	// actually owns plays sound over a picture Chrome never decodes. Measured on
	// that file at 720p, libx264 runs at 1.04x real time -- which is not a
	// margin, it is a coin toss against a stall -- and h264_amf at 4.56x.
	ModeTranscode Mode = "transcode"
)

// Qualities are the heights offered, largest first. Nothing above the source is
// ever listed: upscaling spends a GPU to invent detail that is not there.
var Qualities = []int{1440, 1080, 720, 480, 360}

// AudioAction is what happens to the audio track when remuxing.
type AudioAction string

const (
	AudioCopy      AudioAction = "copy"
	AudioTranscode AudioAction = "transcode"
)

// Decision is the plan for one file.
type Decision struct {
	Mode  Mode        `json:"mode"`
	Audio AudioAction `json:"audio,omitempty"`

	// Reason is shown to the user when Mode is unsupported, and logged
	// otherwise. It exists because "it does not play" is a useless bug report.
	Reason string `json:"reason,omitempty"`

	// VideoRisky marks a remux whose video codec browsers disagree about --
	// HEVC plays in Safari and generally not in Chrome. The remux is still
	// attempted; the flag lets the interface warn rather than look broken.
	VideoRisky bool `json:"video_risky,omitempty"`
}

// browserContainers are the containers a browser will open directly. MKV is
// absent on purpose: no browser plays it, however friendly its contents.
var browserContainers = map[string]bool{
	".mp4": true, ".m4v": true, ".webm": true, ".ogv": true, ".ogg": true,
}

// browserVideo are the video codecs browsers decode broadly.
var browserVideo = map[string]bool{
	"h264": true, "vp8": true, "vp9": true, "av1": true,
}

// riskyVideo are codecs that can be copied into MP4 and will play in some
// browsers but not others.
var riskyVideo = map[string]bool{
	"hevc": true, "h265": true,
}

// browserAudio are the audio codecs browsers decode.
var browserAudio = map[string]bool{
	"aac": true, "mp3": true, "opus": true, "vorbis": true, "flac": true,
}

// DecideByContainer is the decision available without running anything.
//
// This is what keeps the promise that ffmpeg is downloaded on first *need*: a
// library of MP4s is played without ever fetching it. The cost is that an MP4
// hiding an exotic codec is attempted directly and fails in the browser, which
// the player detects and retries as a remux.
func DecideByContainer(path string) Decision {
	ext := strings.ToLower(filepath.Ext(path))
	if browserContainers[ext] {
		return Decision{
			Mode:   ModeDirect,
			Reason: "the container is one browsers open directly",
		}
	}
	return Decision{
		Mode:   ModeRemux,
		Reason: "the container needs rewrapping before a browser will read it",
	}
}

// Decide is the full decision, once the codecs are known.
func Decide(path, videoCodec, audioCodec string) Decision {
	ext := strings.ToLower(filepath.Ext(path))
	video := strings.ToLower(videoCodec)
	audio := strings.ToLower(audioCodec)

	// What the audio needs is decided before the picture, because the answer is
	// the same whichever path the picture takes -- and because an unsupported
	// video has to carry it too.
	//
	// It did not, and that cost the sound. A decision whose mode is unsupported
	// is now the *input* to a transcode (decision 58), so leaving Audio at its
	// zero value meant TranscodeArgs saw no AudioTranscode and emitted
	// `-c:a copy`: an MPEG-2 + AC3 file re-encoded into a picture with no sound.
	// That is decision 1's bug, reappearing on the path opened to fix a
	// different one.
	audioAction := AudioCopy
	if audio != "" && !browserAudio[audio] {
		audioAction = AudioTranscode
	}

	switch {
	case browserVideo[video]:
		// Nothing to do to the picture.
	case riskyVideo[video]:
		// Copyable into MP4, decodable by some browsers. Worth attempting.
	default:
		return Decision{
			Mode:  ModeUnsupported,
			Audio: audioAction,
			Reason: "the video is " + videoCodec +
				", which no browser decodes and which needs an encoder that runs here",
		}
	}

	// A friendly container holding friendly streams needs nothing at all.
	if browserContainers[ext] && browserVideo[video] && (audio == "" || browserAudio[audio]) {
		return Decision{Mode: ModeDirect, Reason: "the file plays as it is"}
	}

	decision := Decision{Mode: ModeRemux, Audio: audioAction, VideoRisky: riskyVideo[video]}
	if audioAction == AudioTranscode {
		// The case decision 1 exists for: an ordinary H.264 + AC3 MKV would
		// otherwise remux into a film with a picture and no sound. Re-encoding
		// audio costs about one core; re-encoding video is the furnace v1
		// refuses to light.
		decision.Audio = AudioTranscode
		decision.Reason = "the audio is " + audioCodec + ", which browsers do not decode"
	} else {
		decision.Reason = "the container needs rewrapping; both streams are copied"
	}
	return decision
}

// RemuxArgs builds the ffmpeg invocation for a decision.
//
// Output is a fragmented MP4 on stdout: fragmented because a plain MP4 needs to
// rewind and write its index at the end, which a pipe cannot do, and empty_moov
// so the browser can start decoding before the file is finished.
func RemuxArgs(path string, d Decision, startSeconds float64) []string {
	return remuxArgs(path, d, startSeconds, nil)
}

// RemuxArgsForAudio maps one absolute ffmpeg stream index. The index comes
// from a server-side inspection record, never directly from an arbitrary path
// or free-form map expression supplied by the browser.
func RemuxArgsForAudio(path string, d Decision, startSeconds float64, streamIndex int) []string {
	return remuxArgs(path, d, startSeconds, &streamIndex)
}

func remuxArgs(path string, d Decision, startSeconds float64, audioStreamIndex *int) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}

	// -ss before -i seeks by keyframe without decoding what came before, which
	// is what makes scrubbing through a two-hour film bearable.
	//
	// -noaccurate_seek is what keeps the sound with the picture, and it is not
	// optional here. ffmpeg's accurate seek trims each stream to the exact
	// timestamp asked for, which it can do for audio because audio is decoded,
	// and cannot do for a video stream being copied: a copy has to start on a
	// keyframe. So an accurate seek to 3600 on this project's own 4K file
	// delivered a picture starting at 3595.467 and sound starting at 3599.98 --
	// measured, by hashing the first output frame against the source and by
	// cross-correlating the output audio against a seek-free decode of it. Four
	// and a half seconds of the sound running ahead of the picture, and up to
	// one keyframe interval; this file's is 10.4 s.
	//
	// Turning accurate seek off makes ffmpeg start every stream at the keyframe
	// instead: measured again, picture at 3595.467 and sound at 3595.47, three
	// milliseconds apart. The cost is that playback resumes up to a keyframe
	// interval before the point that was clicked, and the player's clock -- which
	// counts from what it asked for -- is optimistic by the same amount. That is
	// decision 16's stated granularity, now paid by both streams together rather
	// than by the picture alone.
	//
	// The transcode path deliberately does not do this. Its video is decoded, so
	// an accurate seek trims it exactly like the audio: verified on the same
	// file, where the first transcoded frame is the source frame at 3600 and not
	// the keyframe before it.
	if startSeconds > 0 {
		args = append(args, "-noaccurate_seek", "-ss", formatSeconds(startSeconds))
	}
	audioMap := "0:a:0?"
	if audioStreamIndex != nil {
		audioMap = "0:" + strconv.Itoa(*audioStreamIndex)
	}

	args = append(args,
		"-i", path,
		"-map", "0:v:0",
		"-map", audioMap,
		"-c:v", "copy",
	)

	if d.Audio == AudioTranscode {
		// Stereo at 192k: surround downmixed because a browser is usually two
		// speakers or a pair of headphones, and because copying 5.1 into AAC
		// costs bandwidth nobody hears.
		args = append(args, "-c:a", "aac", "-b:a", "192k", "-ac", "2")
	} else {
		args = append(args, "-c:a", "copy")
	}

	args = append(args,
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)
	return args
}

func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
}

// hdrTransfers are the transfer functions that have to be converted before an
// SDR H.264 encode. Everything else -- including a file whose stream line never
// said -- is left exactly alone.
var hdrTransfers = map[string]bool{
	"smpte2084":    true, // PQ: HDR10, and the base layer under Dolby Vision
	"arib-std-b67": true, // HLG
}

// ToneMap reports whether a source with this transfer function needs converting.
//
// The fault it exists to prevent has no error and no log line. A PQ source
// re-encoded without it hands the encoder BT.2020 code points and tells it they
// are BT.709: the film plays, in the right aspect, at the right speed, and looks
// washed out and grey. Nothing reports a problem, because as far as every part
// of the pipeline is concerned there is not one.
func ToneMap(colorTransfer string) bool {
	return hdrTransfers[strings.ToLower(strings.TrimSpace(colorTransfer))]
}

// ToneMapFilter converts BT.2020 PQ or HLG to BT.709 SDR. Exported because the
// seek-preview builder needs the same conversion: a strip of frames taken off an
// HDR source without it is grey, and a colour pipeline written twice is a colour
// pipeline that drifts.
//
// Four steps, and none of them is optional. zscale linearises against a 100-nit
// display; the float format is what the tonemap filter requires as input; hable
// is the operator, with desat=0 because desaturating highlights on top of an
// already conservative curve makes skies grey; and the last zscale puts the
// result back into BT.709 primaries, matrix and limited range so the encoder is
// told the truth about what it is being given.
//
// It needs zscale, which needs libzimg. The pinned build reports
// --enable-libzimg in -buildconf and lists both filters, which is why this is
// written as a chain rather than as a capability probe.
const ToneMapFilter = "zscale=t=linear:npl=100,format=gbrpf32le," +
	"zscale=p=bt709,tonemap=tonemap=hable:desat=0," +
	"zscale=t=bt709:m=bt709:r=tv,format=yuv420p"

// TranscodeOptions is everything the picture path needs to know beyond the file
// and where to start.
//
// It is a struct rather than more parameters because the list had reached eight
// and every one of them was a bare string or int: a call site could swap the
// encoder and the hardware decoder without the compiler noticing.
type TranscodeOptions struct {
	// Encoder comes from a probe, never from a list -- see ffmpeg.Capabilities.
	Encoder string

	// HWAccel likewise, and the empty string is a real answer meaning "decode on
	// the CPU" -- see ffmpeg.HardwareDecoder for why that is often the right one.
	HWAccel string

	// Height is the rung asked for. Zero keeps the source size, which is what
	// "Original" means for a file whose only problem is a codec the browser
	// refuses.
	Height int

	// SourceHeight is what the file measures. It exists so that a rung of zero
	// can still be given a bitrate sized for the picture it will actually
	// produce: before this, a 4K source kept at its own size was handed the
	// 1080p figure.
	SourceHeight int

	// AudioStreamIndex maps one absolute stream. Nil means the first audio
	// track, whatever it is.
	AudioStreamIndex *int

	// ColorTransfer decides whether the picture has to be tone mapped -- see
	// ToneMap.
	ColorTransfer string
}

// TranscodeArgs re-encodes the picture to H.264 at a target height.
func TranscodeArgs(path string, d Decision, startSeconds float64, o TranscodeOptions) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}

	// An input option, so it has to precede -i. Before -ss as well, so the seek
	// is performed by the accelerated demux path rather than after it.
	if o.HWAccel != "" {
		args = append(args, "-hwaccel", o.HWAccel)
	}

	if startSeconds > 0 {
		args = append(args, "-ss", formatSeconds(startSeconds))
	}

	audioMap := "0:a:0?"
	if o.AudioStreamIndex != nil {
		audioMap = "0:" + strconv.Itoa(*o.AudioStreamIndex)
	}

	args = append(args,
		"-i", path,
		"-map", "0:v:0",
		"-map", audioMap,
		"-c:v", o.Encoder,
	)

	// The picture the encoder will actually produce, which is what a bitrate has
	// to be sized for. A rung of zero means the source keeps its own size.
	picture := resolvedHeight(o.Height, o.SourceHeight)

	var filters []string
	if o.Height > 0 {
		// -2 keeps the aspect ratio and rounds the width to an even number,
		// which every H.264 encoder requires and some crash without.
		//
		// setsar=1 is not decoration. Rounding the width to an even number
		// leaves a remainder, and ffmpeg spends it on the sample aspect ratio to
		// keep the display aspect exact: a 1920x804 source asked for 720 comes
		// out 1720x720 with SAR 2880:2881. The source has square pixels, so the
		// re-encode should too; a fractional SAR is a decoder's problem for a
		// rounding error nobody can see. One even pixel of width is cheaper than
		// non-square pixels.
		filters = append(filters, "scale=-2:"+strconv.Itoa(o.Height)+",setsar=1")
	}
	if ToneMap(o.ColorTransfer) {
		// After the scale rather than before it, and that order is a
		// measurement. Tone mapping is the one thing here a GPU encoder does not
		// help with: zscale converts to linear light in 32-bit float on the CPU,
		// and the cost is the conversion rather than the operator -- hable and
		// mobius came out 1.09x and 1.13x, and -filter_threads 12 changed 1.09x
		// into 1.11x. What did matter was where it sits. Re-encoding this
		// project's own 4K HDR source to 1080p on an AMD desktop:
		//
		//	no tone map (grey picture)   2.79x
		//	scale, then tone map         1.65x
		//	tone map, then scale         1.06x
		//
		// Mapping fewer pixels is 56 per cent faster and costs nothing anyone
		// can see, since the scale happens in the source's own colour space
		// either way.
		filters = append(filters, ToneMapFilter)
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}

	args = append(args,
		"-b:v", targetBitrate(picture),
		"-maxrate", targetBitrate(picture),
		"-bufsize", targetBufsize(picture),
		// Two seconds between keyframes. Seeking a fragmented stream lands on
		// one, so this is how far a seek can miss; longer is cheaper and reads
		// as imprecision.
		"-g", "48",
		// 4:2:0 8-bit. The source may be 10-bit HEVC -- the case that started
		// all this -- and a browser will not decode 10-bit H.264 either.
		"-pix_fmt", "yuv420p",
	)

	if o.Encoder == "libx264" {
		// veryfast is the honest setting for something that has to keep up with
		// playback. Measured at 1.04x real time on a 1080p HEVC source; a
		// slower preset would look better and stall.
		args = append(args, "-preset", "veryfast", "-threads", strconv.Itoa(x264Threads()))
	}

	if d.Audio == AudioTranscode {
		args = append(args, "-c:a", "aac", "-b:a", "192k", "-ac", "2")
	} else {
		args = append(args, "-c:a", "copy")
	}

	return append(args,
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)
}

// targetBitrate is deliberately generous. This is a household network moving
// bytes between two rooms, not a CDN paying for them, and a starved encoder is
// the one thing worse than no quality choice at all.
//
// It used to stop at 1080p and hand every larger picture the same 8 Mb/s, which
// is the 1080p figure applied to four times the pixels. That was not a judgement
// anybody made; it was the top of a table nobody had needed to extend. Measured
// on the 4K source that prompted this: the encoder produced 7734 kb/s against an
// 8 Mb/s ceiling, which is an encoder pressed flat against its limit rather than
// one given room.
//
// height is the picture being produced. Zero means the source's own size, so the
// caller passes that instead -- see resolvedHeight.
func targetBitrate(height int) string {
	switch {
	case height >= 2160:
		return "20M"
	case height >= 1440:
		return "12M"
	case height >= 1080:
		return "8M"
	case height >= 720:
		return "5M"
	case height >= 480:
		return "2500k"
	default:
		return "1200k"
	}
}

// Two seconds of the bitrate above, which is what -g 48 makes a keyframe
// interval worth.
func targetBufsize(height int) string {
	switch {
	case height >= 2160:
		return "40M"
	case height >= 1440:
		return "24M"
	case height >= 1080:
		return "16M"
	case height >= 720:
		return "10M"
	case height >= 480:
		return "5M"
	default:
		return "2400k"
	}
}

// resolvedHeight is the picture the encoder will actually produce: the requested
// rung, or the source's own height when the rung is "leave it alone". A source
// whose height was never measured falls back to 1080, which is what the table
// above did for everything before it learned about 4K.
func resolvedHeight(requested, source int) int {
	if requested > 0 {
		return requested
	}
	if source > 0 {
		return source
	}
	return 1080
}

// AvailableHeights lists what can be offered for a source of this height.
//
// Never above the source, and never a rung so close to it that it is a
// different number for the same picture: a 1080p file offers 720 and below, not
// a "1080p" that re-encodes for nothing.
func AvailableHeights(sourceHeight int) []int {
	var heights []int
	for _, height := range Qualities {
		if sourceHeight > 0 && height >= sourceHeight {
			continue
		}
		heights = append(heights, height)
	}
	return heights
}

// PreferredAudio picks which measured track to play when the viewer has not
// chosen one.
//
// This is a compatibility choice, not a quality one, and the distinction is the
// whole reason it is allowed to exist beside decision 38. Taking track zero
// meant a BluRay rip whose first track is DTS and whose second is already AAC
// got the DTS transcoded on the fly: a core burnt, a 5.1 mix folded to stereo,
// and quality lost, while a browser-ready track sat one index away. Ranking by
// bitrate or channel count would be the forbidden thing; preferring a track the
// browser can actually decode is the same kind of judgement as choosing direct
// play over a remux.
//
// An explicit selection always wins over this, and nothing here reorders what
// the interface shows.
func PreferredAudio[T any](tracks []T, codecOf func(T) string) (T, bool) {
	var zero T
	if len(tracks) == 0 {
		return zero, false
	}
	for _, track := range tracks {
		if browserAudio[strings.ToLower(codecOf(track))] {
			return track, true
		}
	}
	return tracks[0], true
}

// x264Threads caps how many threads libx264 spreads a frame across.
//
// This replaced `-tune zerolatency`, which was the wrong instrument for the
// job. zerolatency exists for a video call: it turns off b-frames and lookahead
// and switches x264 to slice-based threading so that a frame comes out the
// instant it goes in. Theia is not a video call. It writes a fragmented MP4
// into a pipe that a player buffers, and paying for an encoder delay it cannot
// perceive cost both speed and picture.
//
// Measured on a Ryzen AI 9 HX 370, transcoding one minute of 1080p HEVC Main 10
// to 720p, and again with no scale at 1080p:
//
//	                            720p     1080p    first 192 KB
//	veryfast -tune zerolatency  4.52x    3.60x    1108 ms
//	veryfast (threads unbound)  5.26x    4.08x    1400 ms
//	veryfast -threads 8         5.35x       -     1250 ms
//
// Eighteen per cent more throughput, and b-frames and lookahead back, for 142ms
// more before the first fragment. The margin matters where it is thin: the
// machine in decision 58 managed 1.04x real time, which is a coin toss against a
// stall rather than a margin, and this is what it buys there.
//
// The cap is what keeps that 142ms small. x264 holds roughly one frame in flight
// per thread before it emits anything, so an unbounded twenty-four-thread encode
// starts noticeably later than an eight-thread one and finishes no sooner.
// Below eight cores there is nothing to cap, and asking for more threads than
// the machine has only adds context switching.
func x264Threads() int {
	const cap = 8
	if n := runtime.NumCPU(); n < cap {
		return n
	}
	return cap
}
