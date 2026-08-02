package remoteaccess

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type tunnelRuntime struct {
	device   *device.Device
	listener net.Listener
	server   *http.Server
	log      *slog.Logger

	peersMu sync.RWMutex
	peers   map[string]Peer

	closing atomic.Bool
	closed  sync.Once
}

func startTunnel(cfg Config, privateKey wgtypes.Key, peers []Peer, httpPort int,
	handler http.Handler, log *slog.Logger, onError func(error),
) (*tunnelRuntime, error) {
	if handler == nil {
		return nil, errors.New("remote access HTTP handler is not configured")
	}
	tun, tunnelNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(TunnelServerAddress)}, nil, 1420,
	)
	if err != nil {
		return nil, fmt.Errorf("creating userspace WireGuard network: %w", err)
	}

	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	runtime := &tunnelRuntime{
		device: dev,
		log:    log,
		peers:  make(map[string]Peer, len(peers)),
	}
	configuration, err := deviceConfig(privateKey, cfg.ListenPort, peers)
	if err != nil {
		dev.Close()
		return nil, err
	}
	if err := dev.IpcSet(configuration); err != nil {
		dev.Close()
		return nil, fmt.Errorf("configuring WireGuard: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("starting WireGuard: %w", err)
	}

	address := netip.AddrPortFrom(netip.MustParseAddr(TunnelServerAddress), uint16(httpPort))
	listener, err := tunnelNet.ListenTCPAddrPort(address)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("listening inside WireGuard: %w", err)
	}
	runtime.listener = listener
	runtime.replacePeerMap(peers)
	runtime.server = &http.Server{
		Handler:           runtime.protect(handler, httpPort),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	go func() {
		err := runtime.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !runtime.closing.Load() {
			onError(fmt.Errorf("remote HTTP server: %w", err))
		}
	}()
	return runtime, nil
}

func deviceConfig(privateKey wgtypes.Key, port int, peers []Peer) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\nlisten_port=%d\n", hex.EncodeToString(privateKey[:]), port)
	peersConfig, err := peerConfig(peers)
	if err != nil {
		return "", err
	}
	b.WriteString(peersConfig)
	return b.String(), nil
}

func peerConfig(peers []Peer) (string, error) {
	var b strings.Builder
	b.WriteString("replace_peers=true\n")
	prefix := netip.MustParsePrefix(TunnelPrefix)
	for _, peer := range peers {
		key, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return "", fmt.Errorf("parsing public key for peer %d: %w", peer.ID, err)
		}
		address, err := netip.ParseAddr(peer.Address)
		if err != nil || !prefix.Contains(address) || address.String() == TunnelServerAddress {
			return "", fmt.Errorf("invalid tunnel address for peer %d", peer.ID)
		}
		fmt.Fprintf(&b,
			"public_key=%s\nprotocol_version=1\nreplace_allowed_ips=true\nallowed_ip=%s/32\n",
			hex.EncodeToString(key[:]), peer.Address)
	}
	return b.String(), nil
}

func (r *tunnelRuntime) applyPeers(peers []Peer) error {
	configuration, err := peerConfig(peers)
	if err != nil {
		return err
	}
	if err := r.device.IpcSet(configuration); err != nil {
		return fmt.Errorf("applying WireGuard peers: %w", err)
	}
	r.replacePeerMap(peers)
	return nil
}

func (r *tunnelRuntime) replacePeerMap(peers []Peer) {
	r.peersMu.Lock()
	defer r.peersMu.Unlock()
	r.peers = make(map[string]Peer, len(peers))
	for _, peer := range peers {
		r.peers[peer.Address] = peer
	}
}

func (r *tunnelRuntime) peerByAddress(address string) (Peer, bool) {
	r.peersMu.RLock()
	defer r.peersMu.RUnlock()
	peer, ok := r.peers[address]
	return peer, ok
}

func (r *tunnelRuntime) withStats(peers []Peer) []Peer {
	state, err := r.device.IpcGet()
	if err != nil {
		return peers
	}
	byKey := make(map[string]*Peer, len(peers))
	for i := range peers {
		byKey[peers[i].PublicKey] = &peers[i]
	}

	var current *Peer
	for _, line := range strings.Split(state, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "public_key":
			raw, err := hex.DecodeString(value)
			if err != nil || len(raw) != 32 {
				current = nil
				continue
			}
			var parsed wgtypes.Key
			copy(parsed[:], raw)
			current = byKey[parsed.String()]
		case "last_handshake_time_sec":
			if current != nil {
				current.LastHandshakeAt, _ = strconv.ParseInt(value, 10, 64)
			}
		case "rx_bytes":
			if current != nil {
				current.ReceivedBytes, _ = strconv.ParseInt(value, 10, 64)
			}
		case "tx_bytes":
			if current != nil {
				current.TransmittedBytes, _ = strconv.ParseInt(value, 10, 64)
			}
		}
	}
	return peers
}

func (r *tunnelRuntime) close() {
	r.closed.Do(func() {
		r.closing.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if r.server != nil {
			if err := r.server.Shutdown(ctx); err != nil {
				_ = r.server.Close()
			}
		}
		if r.listener != nil {
			_ = r.listener.Close()
		}
		if r.device != nil {
			r.device.Close()
		}
	})
}
