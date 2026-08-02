package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/remoteaccess"
)

func newRemoteAPITestServer(t *testing.T) (http.Handler, *remoteaccess.Service) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(t.Context(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	log := slog.New(slog.DiscardHandler)
	remote := remoteaccess.New(database, dir, 8383, log)
	cfg := config.Default()
	handler := New(Options{
		Config:    &cfg,
		Library:   library.NewService(library.NewStore(database), nil, log),
		State:     db.NewState(database),
		Remote:    remote,
		Web:       fstest.MapFS{},
		Version:   "test",
		KeySource: config.KeyMissing,
		Logger:    log,
	}).Handler()
	if err := remote.Start(t.Context(), handler); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remote.Close)
	return handler, remote
}

func remoteAPIRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRemoteAccessAPIStartsDisabledAndReportsLANSession(t *testing.T) {
	handler, _ := newRemoteAPITestServer(t)
	rec := remoteAPIRequest(t, handler, http.MethodGet, "/api/remote-access", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var status remoteaccess.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.State != "disabled" || status.ListenPort != remoteaccess.DefaultListenPort {
		t.Fatalf("default status = %#v", status)
	}

	rec = remoteAPIRequest(t, handler, http.MethodGet, "/api/remote-access/session", "")
	var session remoteaccess.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Mode != "lan" || session.Peer != nil {
		t.Fatalf("LAN session = %#v", session)
	}
}

func TestRemoteAccessAPIUsesStableValidationErrors(t *testing.T) {
	handler, _ := newRemoteAPITestServer(t)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{http.MethodPut, "/api/remote-access", `{"enabled":true,"listen_port":0,"endpoint":"host:1"}`, 400, "invalid_remote_listen_port"},
		{http.MethodPut, "/api/remote-access", `{"enabled":true,"listen_port":51820,"endpoint":"https://bad"}`, 400, "invalid_remote_endpoint"},
		{http.MethodPut, "/api/remote-access", `{"unknown":true}`, 400, "invalid_remote_access_payload"},
		{http.MethodPost, "/api/remote-access/peers", `{"name":"Phone"}`, 409, "remote_access_disabled"},
		{http.MethodDelete, "/api/remote-access/peers/nope", "", 400, "invalid_remote_peer_id"},
	}
	for _, test := range tests {
		rec := remoteAPIRequest(t, handler, test.method, test.path, test.body)
		if rec.Code != test.status {
			t.Fatalf("%s %s status=%d want=%d body=%s", test.method, test.path, rec.Code, test.status, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["error"] != test.code {
			t.Fatalf("%s %s error=%q want=%q", test.method, test.path, body["error"], test.code)
		}
	}
}

func TestRemoteAccessAPIProvisionsOnceAndRevokes(t *testing.T) {
	handler, _ := newRemoteAPITestServer(t)
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := udp.LocalAddr().(*net.UDPAddr).Port
	udp.Close()

	rec := remoteAPIRequest(t, handler, http.MethodPut, "/api/remote-access", fmtJSON(map[string]any{
		"enabled": true, "listen_port": port,
		"endpoint": net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = remoteAPIRequest(t, handler, http.MethodPost, "/api/remote-access/peers", `{"name":"Phone"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("provision Cache-Control=%q", rec.Header().Get("Cache-Control"))
	}
	var provision remoteaccess.Provision
	if err := json.Unmarshal(rec.Body.Bytes(), &provision); err != nil {
		t.Fatal(err)
	}
	if provision.Peer.ID < 1 || provision.ClientConfig == "" || provision.QRSVG == "" {
		t.Fatalf("provision = %#v", provision)
	}

	rec = remoteAPIRequest(t, handler, http.MethodGet, "/api/remote-access", "")
	if bytes.Contains(rec.Body.Bytes(), []byte("PrivateKey")) {
		t.Fatal("status repeated the one-time private client configuration")
	}
	rec = remoteAPIRequest(t, handler, http.MethodDelete,
		"/api/remote-access/peers/"+strconv.FormatInt(provision.Peer.ID, 10), "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("revoke status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func fmtJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
