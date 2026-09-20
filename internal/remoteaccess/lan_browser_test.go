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

func TestTauriPicturesCanCrossTheLANBrowserBoundary(t *testing.T) {
	h := LANOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), "cinema")
	tests := []struct {
		name, method, path, origin string
		want                       int
	}{
		{"windows webview artwork", http.MethodGet, "/api/images/w780/proof.jpg", "http://tauri.localhost", http.StatusNoContent},
		{"custom scheme artwork", http.MethodGet, "/api/images/w780/proof.jpg", "tauri://localhost", http.StatusNoContent},
		{"hostile artwork", http.MethodGet, "/api/images/w780/proof.jpg", "https://evil.example", http.StatusForbidden},
		{"tauri catalogue", http.MethodGet, "/api/library/movies", "http://tauri.localhost", http.StatusForbidden},
		{"tauri image write", http.MethodPost, "/api/images/w780/proof.jpg", "http://tauri.localhost", http.StatusForbidden},
		// The profile picture: the second image the shell draws, and the one the
		// maintainer reported missing. Its neighbours stay refused, which is what
		// keeps the exception from becoming a way to read the household.
		{"windows webview profile picture", http.MethodGet, "/api/profiles/3/avatar", "http://tauri.localhost", http.StatusNoContent},
		{"custom scheme profile picture", http.MethodGet, "/api/profiles/3/avatar", "tauri://localhost", http.StatusNoContent},
		{"hostile profile picture", http.MethodGet, "/api/profiles/3/avatar", "https://evil.example", http.StatusForbidden},
		{"tauri profile", http.MethodGet, "/api/profiles/3", "http://tauri.localhost", http.StatusForbidden},
		{"tauri profile list", http.MethodGet, "/api/profiles", "http://tauri.localhost", http.StatusForbidden},
		{"tauri profile picture write", http.MethodPut, "/api/profiles/3/avatar", "http://tauri.localhost", http.StatusForbidden},
		{"tauri profile picture traversal", http.MethodGet, "/api/profiles/3/avatar/../x", "http://tauri.localhost", http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "http://127.0.0.1:8395"+test.path, nil)
			req.RemoteAddr = "127.0.0.1:4321"
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			req.Header.Set("Sec-Fetch-Mode", "cors")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != test.want {
				t.Fatalf("status = %d, want %d", rec.Code, test.want)
			}
		})
	}
}
