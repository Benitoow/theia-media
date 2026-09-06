package api

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
)

func TestSettingsDescribePendingActivationAndDoNotPublishInvalidWrites(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Config: cfg, Logger: slog.New(slog.DiscardHandler), KeySource: config.KeyMissing})
	call := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		s.handleUpdateSettings(r, httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body)))
		return r
	}
	response := call(`{"port":8395,"tmdb_api_key":"fixture-key"}`)
	var update settingsUpdateResult
	if err := json.Unmarshal(response.Body.Bytes(), &update); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || !update.RestartRequired || !update.PortChanged {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
	if cfg.Port != config.DefaultPort || cfg.TMDBAPIKey != "" {
		t.Fatal("running configuration mutated")
	}
	if response := call(`{"port":70000}`); response.Code != 400 {
		t.Fatal("invalid port accepted")
	}
	got := s.currentConfig()
	if got.Port != 8395 || got.TMDBAPIKey != "fixture-key" {
		t.Fatal("invalid write changed persisted state")
	}
	state := httptest.NewRecorder()
	s.handleSettings(state, httptest.NewRequest("GET", "/api/settings", nil))
	if strings.Contains(state.Body.String(), "fixture-key") {
		t.Fatal("key exposed")
	}
	var settings settingsResponse
	if err := json.Unmarshal(state.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.RestartRequired || settings.TMDB.Configured {
		t.Fatalf("activation misreported: %+v", settings)
	}
}
