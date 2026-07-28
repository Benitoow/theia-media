package discovery

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	qrcode "github.com/skip2/go-qrcode"
)

func TestQRSVGStructure(t *testing.T) {
	svg, err := QRSVG("http://192.168.1.19:8383")
	if err != nil {
		t.Fatalf("QRSVG returned an unexpected error: %v", err)
	}

	if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("output is not a standalone SVG: %.80s…", svg)
	}
	if !strings.Contains(svg, `shape-rendering="crispEdges"`) {
		// Without this, a browser antialiases the module edges and the symbol
		// gets noticeably harder for a phone to lock onto.
		t.Error("modules are not rendered with crisp edges")
	}
	if !strings.Contains(svg, qrBackground) || !strings.Contains(svg, qrForeground) {
		t.Error("the SVG does not use the documented QR colours")
	}

	// A square viewBox, and one big enough to be a real symbol. Version 1 is
	// 21 modules plus an 8-module quiet zone.
	viewBox := regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`).FindStringSubmatch(svg)
	if viewBox == nil {
		t.Fatal("no viewBox")
	}
	w, _ := strconv.Atoi(viewBox[1])
	h, _ := strconv.Atoi(viewBox[2])
	if w != h {
		t.Errorf("viewBox is %dx%d, want a square", w, h)
	}
	if w < 29 {
		t.Errorf("symbol is %d modules across, too small to be a valid QR code", w)
	}
}

func TestQRSVGHasAQuietZone(t *testing.T) {
	// A scanner needs the empty margin to find the symbol at all. The first
	// rows of the bitmap must therefore be blank, which shows up as no <rect>
	// in the top few rows.
	svg, err := QRSVG("http://192.168.1.19:8383")
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range []string{`y="0"`, `y="1"`, `y="2"`, `y="3"`} {
		if strings.Contains(svg, y) {
			t.Errorf("a module is drawn at %s, so the quiet zone is missing", y)
		}
	}
}

func TestQRSVGLongerContentProducesABiggerSymbol(t *testing.T) {
	// A sanity check that the content actually reaches the encoder rather than
	// a constant being rendered.
	short, err := QRSVG("http://10.0.0.1:80")
	if err != nil {
		t.Fatal(err)
	}
	long, err := QRSVG("http://192.168.100.200:8383/a/very/long/path/for/testing/purposes")
	if err != nil {
		t.Fatal(err)
	}

	size := func(s string) int {
		m := regexp.MustCompile(`viewBox="0 0 (\d+)`).FindStringSubmatch(s)
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if size(long) <= size(short) {
		t.Errorf("long content produced a %d-module symbol, short one %d; content is not reaching the encoder",
			size(long), size(short))
	}
}

// TestQRSVGMatchesTheEncoderExactly is the test that matters.
//
// The encoder is a well-used library and is taken on trust; the SVG rendering
// is code written here, and it is where a bug would actually live -- inverted
// modules, an off-by-one in the run merging, a missing quiet zone. So the SVG
// is parsed back into a module matrix and compared against what the encoder
// produced. A code that survives this scans, or the encoder is wrong.
func TestQRSVGMatchesTheEncoderExactly(t *testing.T) {
	const content = "http://192.168.1.19:8383"

	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	want := code.Bitmap()

	svg, err := QRSVG(content)
	if err != nil {
		t.Fatal(err)
	}

	size := len(want)
	got := make([][]bool, size)
	for i := range got {
		got[i] = make([]bool, size)
	}

	// Every dark run the renderer emitted, painted back onto a blank grid.
	rect := regexp.MustCompile(`<rect x="(\d+)" y="(\d+)" width="(\d+)" height="1"/>`)
	matches := rect.FindAllStringSubmatch(svg, -1)
	if len(matches) == 0 {
		t.Fatal("the SVG contains no module rectangles")
	}
	for _, m := range matches {
		x, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[2])
		w, _ := strconv.Atoi(m[3])
		if y >= size || x+w > size {
			t.Fatalf("a run at (%d,%d) width %d falls outside the %d-module symbol", x, y, w, size)
		}
		for i := x; i < x+w; i++ {
			got[y][i] = true
		}
	}

	for y := range size {
		for x := range size {
			if got[y][x] != want[y][x] {
				t.Fatalf("module (%d,%d) is %v in the SVG and %v in the encoder output",
					x, y, got[y][x], want[y][x])
			}
		}
	}
}

func TestQRSVGRejectsNothingUsable(t *testing.T) {
	// The encoder refuses empty content, which is the right answer: a blank
	// symbol scans successfully and sends the phone nowhere, which is a worse
	// failure than an error the server can report.
	if _, err := QRSVG(""); err == nil {
		t.Error("empty content produced a QR code; a symbol that leads nowhere is worse than an error")
	}
}
