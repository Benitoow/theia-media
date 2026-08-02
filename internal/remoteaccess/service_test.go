package remoteaccess

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Benitoow/theia-media/internal/db"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type testClient struct {
	device *device.Device
	net    *netstack.Net
}

func (c *testClient) close() { c.device.Close() }

func freeUDPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := listener.LocalAddr().(*net.UDPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func buildTestClient(t *testing.T, privateKey, address, serverPublicKey, endpoint string) *testClient {
	t.Helper()
	private, err := wgtypes.ParseKey(privateKey)
	if err != nil {
		t.Fatalf("parsing client key: %v", err)
	}
	server, err := wgtypes.ParseKey(serverPublicKey)
	if err != nil {
		t.Fatalf("parsing server key: %v", err)
	}
	tun, tunnelNet, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr(address)}, nil, 1420)
	if err != nil {
		t.Fatal(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	configuration := fmt.Sprintf(
		"private_key=%s\nlisten_port=0\nreplace_peers=true\n"+
			"public_key=%s\nprotocol_version=1\nreplace_allowed_ips=true\n"+
			"allowed_ip=%s/32\nendpoint=%s\npersistent_keepalive_interval=1\n",
		hex.EncodeToString(private[:]), hex.EncodeToString(server[:]), TunnelServerAddress, endpoint,
	)
	if err := dev.IpcSet(configuration); err != nil {
		dev.Close()
		t.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		t.Fatal(err)
	}
	return &testClient{device: dev, net: tunnelNet}
}

func requestThroughTunnel(client *testClient, httpPort int, method, path, origin string, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	transport := &http.Transport{DialContext: client.net.DialContext, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, method,
		fmt.Sprintf("http://%s:%d%s", TunnelServerAddress, httpPort, path), nil)
	if err != nil {
		return nil, err
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return (&http.Client{Transport: transport}).Do(req)
}

func parseClientFields(t *testing.T, config string) map[string]string {
	t.Helper()
	fields := map[string]string{}
	for _, line := range strings.Split(config, "\n") {
		key, value, ok := strings.Cut(line, " = ")
		if ok {
			fields[key] = value
		}
	}
	for _, key := range []string{"PrivateKey", "Address", "PublicKey", "Endpoint"} {
		if fields[key] == "" {
			t.Fatalf("client config has no %s: %s", key, config)
		}
	}
	fields["Address"] = strings.TrimSuffix(fields["Address"], "/32")
	return fields
}

func TestServiceEndToEndProvisionAuthorizeRevokeAndRestart(t *testing.T) {
	const httpPort = 18383
	ctx := t.Context()
	dataDir := t.TempDir()
	database, err := db.Open(ctx, filepath.Join(dataDir, "theia.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	log := slog.New(slog.DiscardHandler)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, _ := PeerFromRequest(r)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"peer_id": peer.ID,
			"path":    r.URL.Path,
		})
	})

	port := freeUDPPort(t)
	service := New(database, dataDir, httpPort, log)
	if err := service.Start(ctx, handler); err != nil {
		t.Fatalf("starting disabled service: %v", err)
	}
	status, err := service.Update(ctx, Config{
		Enabled: true, ListenPort: port, Endpoint: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	})
	if err != nil {
		t.Fatalf("enabling remote access: %v", err)
	}
	if status.State != "running" || status.ServerPublicKey == "" {
		t.Fatalf("enabled status = %#v", status)
	}
	serverPublicKey := status.ServerPublicKey

	provision, err := service.CreatePeer(ctx, "Télévision du salon")
	if err != nil {
		t.Fatalf("creating peer: %v", err)
	}
	if provision.QRSVG == "" || provision.TunnelURL != "http://10.77.0.1:18383" {
		t.Fatalf("incomplete provision response: %#v", provision)
	}
	fields := parseClientFields(t, provision.ClientConfig)
	if persistedSecret(t, dataDir, fields["PrivateKey"]) {
		t.Fatal("the one-time client private key was persisted in the data directory")
	}

	client := buildTestClient(t, fields["PrivateKey"], fields["Address"], fields["PublicKey"], fields["Endpoint"])
	response, err := requestThroughTunnel(client, httpPort, http.MethodGet, "/api/health", "", 5*time.Second)
	if err != nil {
		t.Fatalf("authorized request: %v", err)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || int64(body["peer_id"].(float64)) != provision.Peer.ID {
		t.Fatalf("authorized response status=%d body=%v", response.StatusCode, body)
	}

	response, err = requestThroughTunnel(client, httpPort, http.MethodGet, "/api/settings", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("remote settings status = %d, want 403", response.StatusCode)
	}
	response.Body.Close()

	response, err = requestThroughTunnel(client, httpPort, http.MethodPut,
		"/api/library/movies/1/progress", "https://evil.example", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin progress status = %d, want 403", response.StatusCode)
	}
	response.Body.Close()

	intruderKey, _ := wgtypes.GeneratePrivateKey()
	intruder := buildTestClient(t, intruderKey.String(), "10.77.0.9", serverPublicKey,
		net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if response, err := requestThroughTunnel(intruder, httpPort, http.MethodGet, "/api/health", "", 1200*time.Millisecond); err == nil {
		response.Body.Close()
		t.Fatal("unknown WireGuard peer reached Theia")
	}
	intruder.close()

	status, err = service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Reachability != "confirmed" || len(status.Peers) != 1 || status.Peers[0].LastHandshakeAt == 0 {
		t.Fatalf("handshake status = %#v", status)
	}
	if err := service.RevokePeer(ctx, provision.Peer.ID); err != nil {
		t.Fatalf("revoking peer: %v", err)
	}
	if response, err := requestThroughTunnel(client, httpPort, http.MethodGet, "/api/health", "", 1200*time.Millisecond); err == nil {
		response.Body.Close()
		t.Fatal("revoked peer reached Theia")
	}
	client.close()
	service.Close()

	restarted := New(database, dataDir, httpPort, log)
	if err := restarted.Start(ctx, handler); err != nil {
		t.Fatalf("restoring remote service: %v", err)
	}
	defer restarted.Close()
	restartedStatus, err := restarted.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if restartedStatus.State != "running" || restartedStatus.ServerPublicKey != serverPublicKey || len(restartedStatus.Peers) != 0 {
		t.Fatalf("restarted status = %#v", restartedStatus)
	}

	second, err := restarted.CreatePeer(ctx, "Portable")
	if err != nil {
		t.Fatal(err)
	}
	secondFields := parseClientFields(t, second.ClientConfig)
	secondClient := buildTestClient(t, secondFields["PrivateKey"], secondFields["Address"],
		secondFields["PublicKey"], secondFields["Endpoint"])
	response, err = requestThroughTunnel(secondClient, httpPort, http.MethodGet, "/api/health", "", 5*time.Second)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("request after restart status=%v err=%v", responseStatus(response), err)
	}
	response.Body.Close()
	secondClient.close()

	// Corrupting the key prevents future enable/start, but it must never trap
	// the owner in an enabled state. LAN disable remains the recovery path.
	if err := os.WriteFile(filepath.Join(dataDir, keyFileName), []byte("corrupt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	disabled, err := restarted.Update(ctx, Config{
		Enabled: false, ListenPort: port, Endpoint: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	})
	if err != nil {
		t.Fatalf("disabling with corrupt key: %v", err)
	}
	if disabled.State != "disabled" {
		t.Fatalf("disabled state = %#v", disabled)
	}
}

func TestEnableFailsClosedWhenUDPPortIsOccupied(t *testing.T) {
	ctx := t.Context()
	dataDir := t.TempDir()
	database, err := db.Open(ctx, filepath.Join(dataDir, "theia.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	occupied, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.LocalAddr().(*net.UDPAddr).Port
	service := New(database, dataDir, 18384, slog.New(slog.DiscardHandler))
	if err := service.Start(ctx, http.NotFoundHandler()); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Update(ctx, Config{
		Enabled: true, ListenPort: port, Endpoint: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	}); err == nil {
		t.Fatal("occupied UDP port enabled remote access")
	}
	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.State != "disabled" {
		t.Fatalf("failed enable persisted or started: %#v", status)
	}
}

func TestFailedReconfigurationRestoresPreviousListener(t *testing.T) {
	const httpPort = 18385
	ctx := t.Context()
	dataDir := t.TempDir()
	database, err := db.Open(ctx, filepath.Join(dataDir, "theia.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	oldPort := freeUDPPort(t)
	service := New(database, dataDir, httpPort, slog.New(slog.DiscardHandler))
	if err := service.Start(ctx, handler); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	oldEndpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(oldPort))
	if _, err := service.Update(ctx, Config{
		Enabled: true, ListenPort: oldPort, Endpoint: oldEndpoint,
	}); err != nil {
		t.Fatalf("enabling original listener: %v", err)
	}

	occupied, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	newPort := occupied.LocalAddr().(*net.UDPAddr).Port
	if _, err := service.Update(ctx, Config{
		Enabled: true, ListenPort: newPort,
		Endpoint: net.JoinHostPort("127.0.0.1", strconv.Itoa(newPort)),
	}); err == nil {
		t.Fatal("occupied replacement port unexpectedly succeeded")
	}

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "running" || status.ListenPort != oldPort || status.Endpoint != oldEndpoint {
		t.Fatalf("original listener was not restored: %#v", status)
	}
	provision, err := service.CreatePeer(ctx, "Restored listener")
	if err != nil {
		t.Fatal(err)
	}
	fields := parseClientFields(t, provision.ClientConfig)
	client := buildTestClient(t, fields["PrivateKey"], fields["Address"], fields["PublicKey"], fields["Endpoint"])
	defer client.close()
	response, err := requestThroughTunnel(client, httpPort, http.MethodGet, "/api/health", "", 5*time.Second)
	if err != nil {
		t.Fatalf("request through restored listener: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("restored listener status = %d, want 204", response.StatusCode)
	}
}

func persistedSecret(t *testing.T, root, secret string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(secret)) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func responseStatus(response *http.Response) any {
	if response == nil {
		return nil
	}
	return response.StatusCode
}
