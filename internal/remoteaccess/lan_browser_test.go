package remoteaccess

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLANBrowserBoundary(t *testing.T) {
	h := LANOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), "cinema")
	for _, tc := range []struct {
		host, origin, site string
		want               int
	}{
		{"localhost:8395", "http://localhost:8395", "same-origin", 204},
		{"cinema.local:8395", "http://cinema.local:8395", "same-origin", 204},
		{"127.0.0.1:8395", "http://evil.example", "cross-site", 403},
		{"evil.example:8395", "", "", 403},
		{"127.0.0.1:8395", "null", "", 403},
		{"localhost:8395", "http://localhost:8397", "same-site", 403},
		{"localhost:8395", "", "", 204},
	} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/api/profiles", nil)
		r.RemoteAddr = "127.0.0.1:4321"
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Sec-Fetch-Site", tc.site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%+v: got %d", tc, w.Code)
		}
		if w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatal("framing allowed")
		}
	}
}

func TestUnknownRemoteRoutesFailClosed(t *testing.T) {
	for _, path := range []string{"/api/library/admin/export", "/api/library/not-a-real-route", "/api/stream/1/admin"} {
		if remoteRouteAllowed("GET", path) {
			t.Errorf("unknown route admitted: %s", path)
		}
	}
}
