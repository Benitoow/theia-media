package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
)

// writeVideo puts a plausible file on disk for the scanner to find.
func writeVideo(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1024)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// searchBody is what /api/library/search answers with, decoded the way the
// interface decodes it: iterating both lists without checking them first.
type searchBody struct {
	Movies    []library.Movie  `json:"movies"`
	Series    []library.Series `json:"series"`
	Truncated bool             `json:"truncated"`
}

func decodeSearch(t *testing.T, res *http.Response) searchBody {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body searchBody
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	return body
}

func TestSearchAlwaysAnswersWithListsRatherThanNull(t *testing.T) {
	// A Go nil slice encodes as JSON null, and the search page iterates both
	// lists straight off the response. Null there is a crash on an empty
	// library, which is the state every new installation is in.
	handler := newTestServer(t, bundle())

	for _, query := range []string{"?q=nothing+matches+this", "?q=", ""} {
		res := get(t, handler, "/api/library/search"+query)
		body := decodeSearch(t, res)
		if body.Movies == nil {
			t.Errorf("searching %q returned null for movies, want an empty list", query)
		}
		if body.Series == nil {
			t.Errorf("searching %q returned null for series, want an empty list", query)
		}
	}
}

func TestSearchFindsAFilmAcrossTheAPI(t *testing.T) {
	handler, service := newTestServerWithLibrary(t, bundle())

	root := t.TempDir()
	writeVideo(t, root, "Le Fabuleux Destin d Amelie Poulain (2001).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	body := decodeSearch(t, get(t, handler, "/api/library/search?q=amelie"))
	if len(body.Movies) != 1 {
		t.Fatalf("matched %d films, want 1", len(body.Movies))
	}

	// A query that matches nothing is an empty answer, not an error: the page
	// says "nothing matches" rather than "the search failed".
	empty := decodeSearch(t, get(t, handler, "/api/library/search?q=zzzzzz"))
	if len(empty.Movies) != 0 {
		t.Errorf("matched %d films for nonsense, want none", len(empty.Movies))
	}
}

func TestDiagnosticsReportWithoutProbingAnythingMissing(t *testing.T) {
	// Built with no ffmpeg manager at all, which is what a machine that has
	// never needed one looks like. The page must still render.
	res := get(t, newTestServer(t, bundle()), "/api/diagnostics")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	var body diagnosticsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if body.FFmpeg.Present {
		t.Error("ffmpeg reported present with no manager configured")
	}
	if body.FFmpeg.Probed {
		t.Error("encoders were reported as probed without an ffmpeg to probe")
	}
	if body.FFmpeg.Encoders == nil {
		t.Error("encoders is null, want an empty list the interface can iterate")
	}
	if body.Library.Watching == nil {
		t.Error("watching is null, want an empty list")
	}
}
