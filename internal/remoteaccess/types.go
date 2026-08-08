// Package remoteaccess exposes Theia through a private, userspace WireGuard
// network. It never creates an operating-system tunnel interface and never
// contacts a control plane.
//
// The public UDP endpoint is obtained from the router itself -- see the
// portmap package -- rather than typed in by hand. That is a change of who is
// asked, not of who is trusted: UPnP and NAT-PMP speak only to the gateway on
// the local network, so there is still no relay, no rendezvous server and no
// endpoint-discovery service anywhere in the path. When the router declines,
// the endpoint goes back to being the owner's to enter.
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

// ErrNoAutomaticEndpoint is returned when remote access was asked to switch
// itself on, the router declined to forward anything, and no endpoint had been
// entered by hand. The specific reason travels in the message and in Status;
// this is the one the HTTP layer matches on.
var ErrNoAutomaticEndpoint = errors.New("the router provided no public endpoint")

// The three ways that can happen, each worth a different sentence to the person
// reading it. They are wrapped alongside ErrNoAutomaticEndpoint, so a caller can
// match either the family or the specific case.
var (
	ErrRouterSilent  = errors.New("no router answered UPnP or NAT-PMP")
	ErrRouterRefused = errors.New("the router refused to forward the port")
	ErrCarrierNAT    = errors.New("this connection is behind carrier-grade NAT")
)

// discoveryError turns a reason code back into the matchable error.
func discoveryError(reason string) error {
	switch reason {
	case DiscoveryRefused:
		return fmt.Errorf("%w: %w", ErrNoAutomaticEndpoint, ErrRouterRefused)
	case DiscoveryNotPublic:
		return fmt.Errorf("%w: %w", ErrNoAutomaticEndpoint, ErrCarrierNAT)
	default:
		return fmt.Errorf("%w: %w", ErrNoAutomaticEndpoint, ErrRouterSilent)
	}
}

// Discovery reason codes. Stable strings, never sentences: the interface owns
// every word a person reads (decision 25), and these are the four outcomes it
// has to be able to explain.
const (
	// DiscoveryNoGateway: nothing answered UPnP or NAT-PMP. Usually the feature
	// is simply switched off in the router's own settings.
	DiscoveryNoGateway = "remote_router_silent"

	// DiscoveryRefused: a router answered and declined -- the port is already
	// forwarded elsewhere, or forwarding is locked down.
	DiscoveryRefused = "remote_router_refused"

	// DiscoveryNotPublic: the mapping worked and leads nowhere, because the
	// address the router holds is itself behind the operator's NAT. No amount
	// of port forwarding reaches through carrier-grade NAT, and saying so is
	// the only useful answer.
	DiscoveryNotPublic = "remote_carrier_nat"
)

// Config is the persisted remote-listener configuration. Endpoint is the
// public host:port placed into newly generated client configurations; it is
// never contacted by Theia itself.
type Config struct {
	Enabled    bool   `json:"enabled"`
	ListenPort int    `json:"listen_port"`
	Endpoint   string `json:"endpoint"`
	UpdatedAt  int64  `json:"updated_at"`

	// Automatic asks the router for the port and the address instead of asking
	// the owner. When it succeeds it fills Endpoint; when the router declines,
	// whatever was typed by hand still stands.
	Automatic bool `json:"automatic"`

	// What the last successful discovery did, so the mapping can be withdrawn
	// on the way out and the panel can name the protocol that answered.
	MappedMethod string `json:"mapped_method,omitempty"`
	MappedPort   int    `json:"mapped_port,omitempty"`
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

	// DiscoveryReason is empty when the router forwarded the port. It is a code
	// and never a sentence; the interface owns the words (decision 25).
	DiscoveryReason string `json:"discovery_reason,omitempty"`

	// EndpointChanged says the public address moved under an already-running
	// tunnel, so devices provisioned before it need a new configuration.
	EndpointChanged bool `json:"endpoint_changed,omitempty"`
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
