package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
)

// homeBody is what /api/library/home answers with, decoded the way the native
// player decodes it: rows into a list it can iterate.
type homeBody struct {
	Hero     *library.Movie `json:"hero"`
	HeroKind string         `json:"hero_kind"`
	Rows     []library.Row  `json:"rows"`
	Total    int            `json:"total"`
}

func decodeHome(t *testing.T, res *http.Response) homeBody {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body homeBody
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	return body
}

func TestTheHomeScreenAlwaysAnswersWithARowListRatherThanNull(t *testing.T) {
	// Same rule as the search endpoint, and it was broken here until an
	// installation with an empty library met the native player: a Go nil slice
	// encodes as JSON null, and the player's rows field is a Vec, so the window
	// said "the home screen could not be loaded" on the one library a fresh
	// installation has. Found by running the released product, not by reading
	// this package.
	handler := newTestServer(t, bundle())

	body := decodeHome(t, get(t, handler, "/api/library/home"))
	if body.Rows == nil {
		t.Error("the home screen returned null for rows, want an empty list")
	}
	if body.Hero != nil {
		t.Errorf("an empty library produced a hero: %+v", body.Hero)
	}
	if body.Total != 0 {
		t.Errorf("an empty library counted %d films", body.Total)
	}
}
