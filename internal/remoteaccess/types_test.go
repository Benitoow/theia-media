package remoteaccess

import (
	"errors"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestValidateEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"203.0.113.10:51820",
		"media.example.net:8443",
		"[2001:db8::10]:51820",
	} {
		if err := validateEndpoint(endpoint); err != nil {
			t.Errorf("validateEndpoint(%q) = %v", endpoint, err)
		}
	}

	for _, endpoint := range []string{
		"", "example.com", "https://example.com:51820", "example.com:0",
		"example.com:70000", "example.com:abc", "example.com:51820/path",
		"-bad.example:51820", "média.example:51820", "0.0.0.0:51820",
	} {
		if err := validateEndpoint(endpoint); !errors.Is(err, ErrInvalidEndpoint) {
			t.Errorf("validateEndpoint(%q) = %v, want ErrInvalidEndpoint", endpoint, err)
		}
	}
}

func TestValidateConfigRequiresEndpointOnlyWhenEnabled(t *testing.T) {
	if _, err := validateConfig(Config{ListenPort: DefaultListenPort}); err != nil {
		t.Fatalf("disabled default config rejected: %v", err)
	}
	if _, err := validateConfig(Config{ListenPort: DefaultListenPort, Endpoint: "broken"}); err != nil {
		t.Fatalf("disabled config must remain recoverable with a malformed endpoint: %v", err)
	}
	if _, err := validateConfig(Config{Enabled: true, ListenPort: DefaultListenPort}); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("enabled config without endpoint = %v, want ErrInvalidEndpoint", err)
	}
	if _, err := validateConfig(Config{ListenPort: 0}); !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("port zero = %v, want ErrInvalidPort", err)
	}
}

func TestValidatePeerName(t *testing.T) {
	got, err := validatePeerName("  Télévision du salon  ")
	if err != nil || got != "Télévision du salon" {
		t.Fatalf("valid name = %q, %v", got, err)
	}
	for _, name := range []string{"", "   ", "bad\nname", strings.Repeat("é", 65)} {
		if _, err := validatePeerName(name); !errors.Is(err, ErrInvalidPeerName) {
			t.Errorf("validatePeerName(%q) = %v, want ErrInvalidPeerName", name, err)
		}
	}
}

func TestClientConfigRoutesOnlyTheia(t *testing.T) {
	client, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	server, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	config := clientConfig(client, "10.77.0.2", server.PublicKey(), "vpn.example:51820")
	for _, wanted := range []string{
		"PrivateKey = " + client.String(),
		"Address = 10.77.0.2/32",
		"PublicKey = " + server.PublicKey().String(),
		"AllowedIPs = 10.77.0.1/32",
		"Endpoint = vpn.example:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(config, wanted) {
			t.Errorf("client config does not contain %q", wanted)
		}
	}
	if strings.Contains(config, "0.0.0.0/0") {
		t.Error("client config routes all internet traffic instead of only Theia")
	}
}
