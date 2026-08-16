package api

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

// Compressing what is worth compressing, and nothing else.
//
// The frontend bundle is 394 KB of JavaScript and CSS, and the film catalogue
// is a JSON array that grows with the library. Both were going over the wire
// verbatim. On the machine serving them that is free; over house Wi-Fi to a
// television, or down a WireGuard tunnel to a phone on mobile data, it is the
// difference between a page that appears and a page that arrives.
//
// The exclusions matter more than the inclusions. A film is already compressed,
// so gzipping it burns CPU to make it bigger; worse, a range request answered
// with a compressed body no longer means what the player asked for, because the
// byte offsets it seeked to are offsets into the file, not into a gzip stream.
// So: never a Range request, and only the handful of text types below.
const (
	// One ethernet frame. Below this, compression cannot save a round trip,
	// and gzip's own header makes very small answers larger.
	compressMinBytes = 1400

	// Speed over ratio. These bodies are served from a local machine to a
	// device in the same house; the last few percent are not worth the CPU on
	// a server that may also be remuxing a film.
	compressLevel = gzip.BestSpeed
)

// compressible reports whether a Content-Type is worth the trouble. Anything
// not named here — video, JPEG, WebP, the sprite sheets — goes out untouched.
func compressible(contentType string) bool {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "application/json",
		"application/javascript",
		"text/javascript",
		"text/html",
		"text/css",
		"text/plain",
		"text/vtt",
		"image/svg+xml",
		"application/manifest+json",
		// TrueType is uncompressed glyph outlines and gzips to roughly the size
		// WOFF2 reaches on its own. Nothing shipped is TrueType today -- both
		// faces are WOFF2 -- but a file dropped into static/ would be, and the
		// rule belongs with the other text types rather than in a surprise.
		// WOFF2 and WOFF are deliberately absent: they carry their own
		// compression, and gzipping them again only costs CPU.
		"font/ttf",
		"application/x-font-ttf",
		"application/font-sfnt":
		return true
	}
	return false
}

var gzipWriters = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, compressLevel)
		return w
	},
}

// compress wraps a handler so that text answers are gzipped when the client
// says it can read them.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptsGzip(r) || r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Caches downstream must know the body varies by encoding, whether or
		// not this particular answer turned out to be compressible.
		w.Header().Add("Vary", "Accept-Encoding")

		cw := &compressWriter{ResponseWriter: w, status: http.StatusOK}
		defer cw.finish()
		next.ServeHTTP(cw, r)
	})
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(name, "gzip") {
			return true
		}
	}
	return false
}

// compressWriter holds the first kilobyte and a half back before deciding.
//
// The decision needs two things the handler has not necessarily provided yet:
// the Content-Type, and some idea of how much body there is. Buffering until
// one of those is known costs a small allocation and means /api/health is not
// gzipped into something larger than it started.
type compressWriter struct {
	http.ResponseWriter

	status      int
	wroteHeader bool

	decided bool
	gz      *gzip.Writer
	buf     []byte
}

func (c *compressWriter) WriteHeader(status int) {
	// Recorded, not sent: the header cannot go out until it is known whether
	// Content-Encoding belongs in it.
	if !c.wroteHeader {
		c.status = status
		c.wroteHeader = true
	}
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if c.decided {
		if c.gz != nil {
			return c.gz.Write(p)
		}
		return c.ResponseWriter.Write(p)
	}

	c.buf = append(c.buf, p...)
	if len(c.buf) >= compressMinBytes {
		if err := c.decide(true); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// decide commits to an encoding, sends the header and flushes the buffer.
// large says whether the body has already passed the threshold; a body that
// ended below it is sent as it is.
func (c *compressWriter) decide(large bool) error {
	c.decided = true

	header := c.ResponseWriter.Header()
	worth := large &&
		compressible(header.Get("Content-Type")) &&
		header.Get("Content-Encoding") == "" &&
		c.status != http.StatusNoContent &&
		c.status != http.StatusNotModified &&
		c.status != http.StatusPartialContent

	if worth {
		header.Set("Content-Encoding", "gzip")
		// The length that was announced was the length before compression, and
		// no useful one is known until the body has been written.
		header.Del("Content-Length")
		c.gz = gzipWriters.Get().(*gzip.Writer)
		c.gz.Reset(c.ResponseWriter)
	}

	c.ResponseWriter.WriteHeader(c.status)

	if len(c.buf) == 0 {
		return nil
	}
	var err error
	if c.gz != nil {
		_, err = c.gz.Write(c.buf)
	} else {
		_, err = c.ResponseWriter.Write(c.buf)
	}
	c.buf = nil
	return err
}

// finish sends whatever is still held back. Always called, including for the
// handlers that write no body at all.
func (c *compressWriter) finish() {
	if !c.decided {
		_ = c.decide(false)
	}
	if c.gz != nil {
		_ = c.gz.Close()
		c.gz.Reset(nil)
		gzipWriters.Put(c.gz)
		c.gz = nil
	}
}

// Unwrap keeps http.ResponseController working through this layer, as the
// status recorder beside it already does.
func (c *compressWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// Flush pushes a partially built body out. Nothing in Theia streams text
// today, but a wrapper that silently swallows Flush is the kind of thing that
// is discovered much later, from the far end of a stalled connection.
func (c *compressWriter) Flush() {
	if !c.decided {
		_ = c.decide(len(c.buf) >= compressMinBytes)
	}
	if c.gz != nil {
		_ = c.gz.Flush()
	}
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
