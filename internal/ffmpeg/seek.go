package ffmpeg

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Where a seek actually lands.
//
// A remux copies the video stream, and a copy can only start on a keyframe. The
// remux therefore passes -noaccurate_seek so the sound starts on that keyframe
// too rather than at the requested time (decision 90) -- which keeps the film
// together and leaves the player counting from a moment that is up to one
// keyframe interval later than the picture it is showing. On this project's own
// 4K file that interval is 10.4 s, and the error is always in the same
// direction: the clock reads ahead of what is on screen, and saves that number
// as the resume position.
//
// This answers the question the player cannot: given the time somebody asked
// for, where does the stream really begin.

// ErrNoKeyframe means ffmpeg produced no packet for the seek -- past the end of
// the file, or a stream it could not index. The caller carries on with the
// requested time, which is what it did before this existed.
var ErrNoKeyframe = errors.New("ffmpeg: no keyframe found for that position")

var (
	// framecrc prints the stream timebase in its header, and it must be read
	// rather than assumed: Matroska reports 1/1000 here while the video stream's
	// own rate is 24000/1001, and using the wrong one puts the answer out by a
	// factor of forty.
	timebasePattern = regexp.MustCompile(`^#tb 0:\s*(\d+)/(\d+)`)
	// "0,      3595384,      3595384,       41,   123456, 0x1234abcd"
	packetPattern = regexp.MustCompile(`^0,\s*(-?\d+),`)
)

// KeyframeAt reports the position, in seconds, where a remux seeking to
// `seconds` will actually begin.
//
// It costs one short ffmpeg run -- measured at about 200 ms on a 13.9 GB file,
// consistently, because -c copy and one frame means an index seek and a single
// packet read rather than a decode. That is why the player asks for it
// *alongside* the stream rather than before it: the picture is never made to
// wait for this.
func (m *Manager) KeyframeAt(ctx context.Context, path string, seconds float64) (float64, error) {
	binary, err := m.Path(ctx)
	if err != nil {
		return 0, err
	}

	// A ceiling, because this is a comfort and must never become the reason a
	// seek hangs. A file whose index makes this slow simply does not get a
	// corrected clock.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error",
		// -copyts is the whole point: without it ffmpeg rebases the answer to
		// zero and reports where the packet sits in the output rather than in
		// the film.
		"-copyts",
		"-noaccurate_seek",
		"-ss", strconv.FormatFloat(seconds, 'f', 3, 64),
		"-i", path,
		"-map", "0:v:0",
		"-c", "copy",
		"-frames:v", "1",
		"-f", "framecrc", "-",
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return parseKeyframe(string(output))
}

// parseKeyframe reads framecrc's output. Split out so the parsing -- the part
// that is a guess about someone else's text format -- can be tested without
// spawning anything.
func parseKeyframe(output string) (float64, error) {
	num, den := 0, 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if match := timebasePattern.FindStringSubmatch(line); match != nil {
			num, _ = strconv.Atoi(match[1])
			den, _ = strconv.Atoi(match[2])
			continue
		}
		match := packetPattern.FindStringSubmatch(line)
		if match == nil || den == 0 {
			continue
		}
		pts, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		return float64(pts) * float64(num) / float64(den), nil
	}
	return 0, ErrNoKeyframe
}
