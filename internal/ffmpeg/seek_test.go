package ffmpeg

import (
	"errors"
	"math"
	"testing"
)

// The timebase is read, not assumed, and this is the fixture that proves why.
//
// Copied from a real run against the maintainer's 4K file. Matroska reports
// 1/1000 on the copied video stream while the picture itself runs at 24000/1001;
// assuming the frame rate here reported 149957 seconds instead of 3595.
func TestParseKeyframeReadsTheTimebaseRatherThanGuessingIt(t *testing.T) {
	const output = `#format: frame checksums
#version: 2
#hash: CRC32
#tb 0: 1/1000
#media_type 0: video
#codec_id 0: hevc
0,      3595384,      3595384,       41,   462891, 0x8d2d3fd5
`
	got, err := parseKeyframe(output)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-3595.384) > 0.001 {
		t.Errorf("keyframe = %v, want 3595.384", got)
	}
}

// A stream whose timebase really is the frame rate has to work too.
func TestParseKeyframeHandlesAFrameRateTimebase(t *testing.T) {
	const output = `#tb 0: 1001/24000
#media_type 0: video
0,        86205,        86205,        1,   123456, 0xdeadbeef
`
	got, err := parseKeyframe(output)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-3595.467) > 0.01 {
		t.Errorf("keyframe = %v, want about 3595.467", got)
	}
}

// Nothing to report is a normal answer, not a failure to hide: past the end of
// the film, or a stream ffmpeg could not index. The caller keeps the time it
// asked for.
func TestParseKeyframeSaysSoWhenThereIsNoPacket(t *testing.T) {
	for _, output := range []string{
		"",
		"#tb 0: 1/1000\n#media_type 0: video\n",
		"0,      3595384,      3595384,       41,   462891, 0x8d2d3fd5\n", // no timebase
	} {
		if _, err := parseKeyframe(output); !errors.Is(err, ErrNoKeyframe) {
			t.Errorf("output %q gave %v, want ErrNoKeyframe", output, err)
		}
	}
}
