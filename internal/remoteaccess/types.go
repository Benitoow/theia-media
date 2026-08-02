// Package remoteaccess exposes Theia through a private, userspace WireGuard
// network. It never creates an operating-system tunnel interface and never
// contacts a control plane: the public UDP endpoint remains the owner's
// responsibility.
package remoteaccess

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultListenPort is WireGuard's conventional UDP port.
	DefaultListenPort = 51820

	// TunnelServerAddress is deliberately fixed. Clients route this single /32
	// through WireGuard, so it wins even if their current LAN uses the same /24.
	TunnelServerAddress = "10.77.0.1"
	TunnelPrefix        = "10.77.0.0/24"

	// MaxPeers keeps a mistaken or hostile local caller from turning the
	// embedded VPN into an unbounded key store. This is a household server, not
	// a commercial concentrator wearing slippers.
	MaxPeers = 32

	keyFileName = "remote-access.key"
)

var (
	ErrDisabled        = errors.New("remote access is disabled")
	ErrNotReady        = errors.New("remote access is not running")
	ErrUnavailable     = errors.New("remote access is unavailable")
	ErrPeerNotFound    = errors.New("remote access peer not found")
	ErrPeerLimit       = errors.New("remote access peer limit reached")
	ErrInvalidPort     = errors.New("invalid remote access port")
	ErrInvalidEndpoint = errors.New("invalid remote access endpoint")
	ErrInvalidPeerName = errors.New("invalid remote access peer name")
)

// Config is the persisted remote-listener configuration. Endpoint is the
// public host:port placed into newly generated client configurations; it is
// never contacted by Theia itself.
type Config struct {
	Enabled    bool   `json:"enabled"`
	ListenPort int    `json:"listen_port"`
	Endpoint   string `json:"endpoint"`
	UpdatedAt  int64  `json:"updated_at"`
}

// Peer is one active client identity. PublicKey and Address are not secrets;
// the private key exists only in the one provisioning response.
type Peer struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	PublicKey        string `json:"public_key"`
	Address          string `json:"address"`
	CreatedAt        int64  `json:"created_at"`
	LastHandshakeAt  int64  `json:"last_handshake_at,omitempty"`
	ReceivedBytes    int64  `json:"received_bytes"`
	TransmittedBytes int64  `json:"transmitted_bytes"`
}

// Status is safe for the local management API. It deliberately has no key
// path and no server private key.
type Status struct {
	Config
	State           string `json:"state"`
	Reason          string `json:"reason,omitempty"`
	TunnelURL       string `json:"tunnel_url"`
	ServerPublicKey string `json:"server_public_key,omitempty"`
	Reachability    string `json:"reachability"`
	Peers           []Peer `json:"peers"`
}

// Provision is returned exactly once when a peer is created. ClientConfig and
// QRSVG contain the client private key and must never be logged or persisted.
type Provision struct {
	Peer         Peer   `json:"peer"`
	ClientConfig string `json:"client_config"`
	QRSVG        string `json:"qr_svg"`
	TunnelURL    string `json:"tunnel_url"`
}

// Session identifies how the current HTTP request reached Theia.
type Session struct {
	Mode string `json:"mode"`
	Peer *Peer  `json:"peer,omitempty"`
}

func validateConfig(cfg Config) (Config, error) {
	if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
		return Config{}, ErrInvalidPort
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	if cfg.Enabled {
		if err := validateEndpoint(cfg.Endpoint); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func validateEndpoint(endpoint string) error {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" {
		return ErrInvalidEndpoint
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return ErrInvalidEndpoint
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.IsUnspecified() {
			return ErrInvalidEndpoint
		}
		return nil
	}
	if len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return ErrInvalidEndpoint
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ErrInvalidEndpoint
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
				!(r >= '0' && r <= '9') && r != '-' {
				return ErrInvalidEndpoint
			}
		}
	}
	return nil
}

func validatePeerName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 {
		return "", ErrInvalidPeerName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrInvalidPeerName
		}
	}
	return name, nil
}

func tunnelURL(httpPort int) string {
	return fmt.Sprintf("http://%s:%d", TunnelServerAddress, httpPort)
}
