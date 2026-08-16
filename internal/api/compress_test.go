package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve runs one request through the compression middleware in front of a
// handler that answers with the given type and body.
func serve(t *testing.T, req *http.Request, contentType string, body []byte, extra func(http.ResponseWriter)) *http.Response {
	t.Helper()
	handler := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		if extra != nil {
			extra(w)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Result()
}

func gzipRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	return req
}

func TestALargeCatalogueTravelsCompressed(t *testing.T) {
	// The film list is the payload this exists for: it grows with the library
	// and is fetched by every device in the house.
	body := []byte(`{"movies":[` + strings.Repeat(`{"title":"Heat","year":1995},`, 400) + `null]}`)

	res := serve(t, gzipRequest("/api/library/movies"), "application/json; charset=utf-8", body, nil)
	defer res.Body.Close()

	if enc := res.Header.Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", enc)
	}
	if vary := res.Header.Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %q, want it to name Accept-Encoding", vary)
	}

	reader, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("the body did not decompress: %v", err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading the decompressed body: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Error("the decompressed body is not what the handler wrote")
	}
}

func TestARangeRequestIsNeverCompressed(t *testing.T) {
	// The one that would be a real bug rather than a missed optimisation: a
	// player seeks by byte offset into the file, and those offsets mean nothing
	// once the body has been through gzip.
	req := gzipRequest("/api/stream/7")
	req.Header.Set("Range", "bytes=1048576-2097152")

	res := serve(t, req, "text/plain", bytes.Repeat([]byte("x"), 64*1024), nil)
	defer res.Body.Close()

	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q for a range request, want none", enc)
	}
}

func TestAFilmIsNotCompressed(t *testing.T) {
	// Video is already compressed. Doing it again costs CPU on the machine that
	// may be remuxing at the same time, and makes the body no smaller.
	for _, kind := range []string{"video/mp4", "video/x-matroska", "image/jpeg", "image/webp"} {
		res := serve(t, gzipRequest("/api/stream/7"), kind, bytes.Repeat([]byte("x"), 64*1024), nil)
		res.Body.Close()
		if enc := res.Header.Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding = %q for %s, want none", enc, kind)
		}
	}
}

func TestASmallAnswerIsLeftAlone(t *testing.T) {
	// gzip's own header is longer than /api/health's whole body.
	res := serve(t, gzipRequest("/api/health"), "application/json", []byte(`{"status":"ok"}`), nil)
	defer res.Body.Close()

	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q for a fifteen-byte body, want none", enc)
	}
	got, _ := io.ReadAll(res.Body)
	if string(got) != `{"status":"ok"}` {
		t.Errorf("body = %q, want it through untouched", got)
	}
}

func TestAClientThatCannotReadGzipGetsPlainBytes(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 8192)
	req := httptest.NewRequest(http.MethodGet, "/api/library/movies", nil)

	res := serve(t, req, "application/json", body, nil)
	defer res.Body.Close()

	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q with no Accept-Encoding, want none", enc)
	}
	got, _ := io.ReadAll(res.Body)
	if !bytes.Equal(got, body) {
		t.Error("the body changed for a client that never asked for gzip")
	}
}

func TestAnAlreadyEncodedBodyIsNotEncodedTwice(t *testing.T) {
	res := serve(t, gzipRequest("/api/library/movies"), "application/json",
		bytes.Repeat([]byte("a"), 8192),
		func(w http.ResponseWriter) { w.Header().Set("Content-Encoding", "br") })
	defer res.Body.Close()

	if enc := res.Header.Get("Content-Encoding"); enc != "br" {
		t.Fatalf("Content-Encoding = %q, want the handler's own br", enc)
	}
}

func TestTheAnnouncedLengthIsDroppedWhenTheBodyIsCompressed(t *testing.T) {
	// A Content-Length measured before compression describes a body that is no
	// longer being sent, and a client that believes it hangs waiting for bytes
	// that never come.
	body := bytes.Repeat([]byte("a"), 8192)
	res := serve(t, gzipRequest("/api/library/movies"), "application/json", body,
		func(w http.ResponseWriter) { w.Header().Set("Content-Length", "8192") })
	defer res.Body.Close()

	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("the body was not compressed, so this proves nothing")
	}
	if length := res.Header.Get("Content-Length"); length == "8192" {
		t.Error("the uncompressed length was left on a compressed body")
	}
}

func TestCompressibleReadsTheTypeAndNotItsParameters(t *testing.T) {
	for _, want := range []string{"application/json", "application/json; charset=utf-8", "TEXT/CSS", "text/vtt; charset=utf-8"} {
		if !compressible(want) {
			t.Errorf("compressible(%q) = false, want true", want)
		}
	}
	// The shipped faces are TrueType, which is uncompressed outlines.
	for _, want := range []string{"font/ttf", "application/x-font-ttf"} {
		if !compressible(want) {
			t.Errorf("compressible(%q) = false, want true", want)
		}
	}
	// WOFF and WOFF2 carry their own compression; doing it twice only costs CPU.
	for _, wrong := range []string{"video/mp4", "image/jpeg", "application/octet-stream",
		"font/woff", "font/woff2", ""} {
		if compressible(wrong) {
			t.Errorf("compressible(%q) = true, want false", wrong)
		}
	}
}
