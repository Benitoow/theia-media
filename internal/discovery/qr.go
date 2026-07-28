package discovery

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// QR colours. Dark modules on a light field, which is the orientation every
// scanner handles. Inverted codes work on most modern phones and fail on
// exactly the old one somebody will try, and a QR code that does not scan is
// worse than no QR code at all.
const (
	qrForeground = "#0B0A09" // --color-ink
	qrBackground = "#EDE7DC" // --color-bone
)

// QRSVG renders a QR code for content as a standalone SVG.
//
// SVG rather than PNG so it stays sharp at whatever size the page gives it --
// this is meant to be pointed at with a camera, and a soft QR code is a QR code
// that takes three attempts.
//
// Medium error correction: enough redundancy for a screen photographed at an
// angle, without inflating the symbol so much that the modules get small.
func QRSVG(content string) (string, error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("encoding the QR code: %w", err)
	}

	// Bitmap already carries the quiet zone the specification requires; a
	// scanner needs that empty margin to find the symbol at all.
	bitmap := code.Bitmap()
	size := len(bitmap)
	if size == 0 {
		return "", fmt.Errorf("the QR encoder produced an empty symbol")
	}

	var b strings.Builder
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" `+
			`shape-rendering="crispEdges" role="img" aria-label="QR code">`,
		size, size)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, size, size, qrBackground)

	// Consecutive dark modules in a row become one rectangle. A 33-module
	// symbol is over a thousand modules; merging runs cuts that by roughly
	// half and keeps the markup small enough to inline in a JSON response.
	fmt.Fprintf(&b, `<g fill="%s">`, qrForeground)
	for y, row := range bitmap {
		x := 0
		for x < len(row) {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < len(row) && row[x] {
				x++
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="1"/>`, start, y, x-start)
		}
	}
	b.WriteString(`</g></svg>`)
	return b.String(), nil
}
