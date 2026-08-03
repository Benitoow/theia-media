package remoteaccess

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestLANOnlyAcceptsPrivateAndLoopbackAddresses(t *testing.T) {
	handler := LANOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, address := range []string{
		"127.0.0.1:1234", "192.168.1.20:1234", "10.0.0.5:1234",
		"[::1]:1234", "[fd00::2]:1234", "[fe80::2%eth0]:1234",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://theia/", nil)
		req.RemoteAddr = address
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("address %s status = %d, want 204", address, rec.Code)
		}
	}
}

func TestLANOnlyRejectsPublicAddressAndIgnoresForwardedHeaders(t *testing.T) {
	handler := lanOnlyWithPrefixes(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("public request reached the LAN handler")
	}), nil)
	req := httptest.NewRequest(http.MethodGet, "http://theia/", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.10")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertRemoteError(t, rec, http.StatusForbidden, "lan_access_required")
}

func TestLANOnlyAcceptsAnOnLinkGlobalIPv6Prefix(t *testing.T) {
	handler := lanOnlyWithPrefixes(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), []netip.Prefix{netip.MustParsePrefix("2001:db8:42::/64")})
	req := httptest.NewRequest(http.MethodGet, "http://theia/", nil)
	req.RemoteAddr = "[2001:db8:42::99]:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("on-link IPv6 status = %d, want 204", rec.Code)
	}
}

func TestRemoteProtectionUsesPeerHostRouteAndOrigin(t *testing.T) {
	runtime := &tunnelRuntime{peers: map[string]Peer{
		"10.77.0.2": {ID: 7, Name: "Salon", Address: "10.77.0.2"},
	}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := PeerFromRequest(r)
		if !ok || peer.ID != 7 {
			t.Errorf("authenticated peer = %#v, %v", peer, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := runtime.protect(next, 8383)

	tests := []struct {
		name       string
		method     string
		path       string
		remoteAddr string
		host       string
		origin     string
		fetchSite  string
		fetchMode  string
		status     int
		code       string
	}{
		{name: "catalogue", method: http.MethodGet, path: "/api/library/movies", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 204},
		{name: "static", method: http.MethodGet, path: "/films/12", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 204},
		{name: "same origin progress", method: http.MethodPut, path: "/api/library/movies/12/progress", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", origin: "http://10.77.0.1:8383", status: 204},
		{name: "native progress", method: http.MethodPut, path: "/api/library/episodes/4/progress", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 204},
		{name: "settings forbidden", method: http.MethodGet, path: "/api/settings", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 403, code: "remote_access_forbidden"},
		{name: "onboarding addresses forbidden", method: http.MethodGet, path: "/api/onboarding", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 403, code: "remote_access_forbidden"},
		{name: "peer administration forbidden", method: http.MethodGet, path: "/api/remote-access", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 403, code: "remote_access_forbidden"},
		{name: "scan forbidden", method: http.MethodPost, path: "/api/library/scan", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", status: 403, code: "remote_access_forbidden"},
		{name: "cross origin", method: http.MethodPut, path: "/api/library/movies/12/progress", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", origin: "https://evil.example", status: 403, code: "remote_origin_forbidden"},
		{name: "cross site", method: http.MethodPut, path: "/api/library/movies/12/progress", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", origin: "http://10.77.0.1:8383", fetchSite: "cross-site", status: 403, code: "remote_origin_forbidden"},
		{name: "cross site video", method: http.MethodGet, path: "/api/stream/12", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", fetchSite: "cross-site", fetchMode: "no-cors", status: 403, code: "remote_origin_forbidden"},
		{name: "cross origin catalogue", method: http.MethodGet, path: "/api/library/movies", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", origin: "https://evil.example", status: 403, code: "remote_origin_forbidden"},
		{name: "cross site top-level navigation", method: http.MethodGet, path: "/films", remoteAddr: "10.77.0.2:4000", host: "10.77.0.1:8383", fetchSite: "cross-site", fetchMode: "navigate", status: 204},
		{name: "dns rebinding", method: http.MethodGet, path: "/api/library/movies", remoteAddr: "10.77.0.2:4000", host: "evil.example", status: 421, code: "remote_host_invalid"},
		{name: "unknown peer", method: http.MethodGet, path: "/api/library/movies", remoteAddr: "10.77.0.3:4000", host: "10.77.0.1:8383", status: 403, code: "remote_peer_unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "http://10.77.0.1:8383"+test.path, nil)
			req.RemoteAddr = test.remoteAddr
			req.Host = test.host
			if test.origin != "" {
				req.Header.Set("Origin", test.origin)
			}
			if test.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", test.fetchSite)
			}
			if test.fetchMode != "" {
				req.Header.Set("Sec-Fetch-Mode", test.fetchMode)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Fatalf("X-Frame-Options = %q, want DENY", got)
			}
			if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
				t.Fatalf("Cross-Origin-Resource-Policy = %q, want same-origin", got)
			}
			if test.code == "" {
				if rec.Code != test.status {
					t.Fatalf("status = %d, want %d; body=%s", rec.Code, test.status, rec.Body.String())
				}
				return
			}
			assertRemoteError(t, rec, test.status, test.code)
		})
	}
}

func assertRemoteError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, status, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if body["error"] != code {
		t.Fatalf("error = %q, want %q", body["error"], code)
	}
}

// M2 put a profile on every progress write, so a remote device has to be able
// to read the list and the pictures. Everything that *manages* the household
// stays on the LAN, and this is the test that keeps it there.
func TestRemoteAllowlistCoversProfileReadsAndRefusesProfileManagement(t *testing.T) {
	allowed := []struct{ method, path string }{
		{http.MethodGet, "/api/profiles"},
		{http.MethodGet, "/api/profiles/3/avatar"},
		{http.MethodPut, "/api/library/movies/7/progress"},
		{http.MethodDelete, "/api/library/episodes/7/progress"},
	}
	for _, tc := range allowed {
		if !remoteRouteAllowed(tc.method, tc.path) {
			t.Errorf("%s %s was refused remotely, want allowed", tc.method, tc.path)
		}
	}

	refused := []struct{ method, path string }{
		{http.MethodPost, "/api/profiles"},
		{http.MethodPatch, "/api/profiles/3"},
		{http.MethodDelete, "/api/profiles/3"},
		{http.MethodPut, "/api/profiles/3/avatar"},
		{http.MethodDelete, "/api/profiles/3/avatar"},
		{http.MethodGet, "/api/settings"},
		{http.MethodPost, "/api/library/scan"},
		{http.MethodGet, "/api/remote-access"},
	}
	for _, tc := range refused {
		if remoteRouteAllowed(tc.method, tc.path) {
			t.Errorf("%s %s was allowed remotely, want refused", tc.method, tc.path)
		}
	}
}
