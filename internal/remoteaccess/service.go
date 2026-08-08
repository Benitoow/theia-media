package remoteaccess

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/Benitoow/theia-media/internal/discovery"
	"github.com/Benitoow/theia-media/internal/portmap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Service owns persisted peers and the optional userspace WireGuard runtime.
// All mutations are serialized so SQLite and the live peer set cannot race in
// opposite directions.
type Service struct {
	mu       sync.Mutex
	store    *store
	keyPath  string
	httpPort int
	log      *slog.Logger

	handler    http.Handler
	runtime    *tunnelRuntime
	lastReason string

	// The router side. mapping is what the gateway last granted; mapped says
	// whether there is one to withdraw. discoveryReason is separate from
	// lastReason because a router that declined is not a listener that failed:
	// the tunnel can be running perfectly and still be unreachable from
	// outside, and those are two different sentences.
	mapping         portmap.Mapping
	mapped          bool
	discoveryReason string
	endpointChanged bool
	renewCancel     context.CancelFunc
}

func New(database *sql.DB, dataDir string, httpPort int, log *slog.Logger) *Service {
	return &Service{
		store:    newStore(database),
		keyPath:  filepath.Join(dataDir, keyFileName),
		httpPort: httpPort,
		log:      log,
	}
}

// Start attaches the already-built Theia handler and restores an enabled
// tunnel. Failure is returned to be logged, but callers should keep the LAN
// server alive: local access is the recovery path for a bad endpoint or port.
func (s *Service) Start(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler

	cfg, err := s.store.config(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		s.lastReason = ""
		return nil
	}
	// A domestic address is renumbered by a router reboot, and a leased mapping
	// is forgotten by one. Both happen while nobody is watching, so an enabled
	// automatic configuration asks again on every start rather than trusting
	// what it wrote down last time.
	if cfg.Automatic {
		mapped, reason := s.mapPortLocked(ctx, cfg)
		s.discoveryReason = reason
		cfg = mapped
		if reason == "" {
			if err := s.store.saveConfig(ctx, cfg); err != nil {
				s.log.Warn("recording the discovered endpoint failed", "error", err)
			}
		}
	}

	cfg, err = validateConfig(cfg)
	if err != nil {
		s.lastReason = "remote_config_invalid"
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		return err
	}
	key, err := loadOrCreatePrivateKey(s.keyPath)
	if err != nil {
		s.lastReason = "remote_key_unavailable"
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if err := s.startLocked(cfg, key, peers); err != nil {
		s.lastReason = "remote_listen_failed"
		return err
	}
	s.startRenewalLocked(cfg)
	return nil
}

// Update validates, applies and persists the complete remote configuration.
// Reconfiguration fails closed: if the new listener cannot start and the old
// one cannot be restored, remote access stays down while LAN access survives.
func (s *Service) Update(ctx context.Context, cfg Config) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.store.config(ctx)
	if err != nil {
		return Status{}, err
	}

	// Discovery runs before validation, because with automatic on there is
	// nothing to validate yet: the endpoint is what the router is about to say.
	// Turning it off releases the mapping rather than leaving a hole open in
	// somebody's router after they switched the feature off.
	s.discoveryReason = ""
	switch {
	case cfg.Enabled && cfg.Automatic:
		if old.Enabled && old.Automatic && s.mapped && old.ListenPort == cfg.ListenPort {
			// Already forwarded and unchanged: saving an unrelated setting must
			// not make somebody wait on the network again.
			cfg.Endpoint = s.mapping.Endpoint()
			cfg.MappedMethod = s.mapping.Method
			cfg.MappedPort = s.mapping.ExternalPort
		} else {
			s.releaseMappingLocked()
			cfg, s.discoveryReason = s.mapPortLocked(ctx, cfg)
		}
	case !cfg.Enabled:
		s.stopRenewalLocked()
		s.releaseMappingLocked()
		cfg.MappedMethod = ""
		cfg.MappedPort = 0
	}

	validated, err := validateConfig(cfg)
	if err != nil {
		if s.discoveryReason != "" {
			// The endpoint is empty because the router declined, not because
			// somebody typed something wrong. Those get different sentences.
			return Status{}, discoveryError(s.discoveryReason)
		}
		return Status{}, err
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		return Status{}, err
	}

	// Endpoint changes affect only future client configurations. Restarting the
	// encrypted listener would cut active playback for no technical reason.
	if old.Enabled && validated.Enabled && old.ListenPort == validated.ListenPort && s.runtime != nil {
		if err := s.store.saveConfig(ctx, validated); err != nil {
			return Status{}, err
		}
		saved, err := s.store.config(ctx)
		if err != nil {
			return Status{}, err
		}
		return s.statusLocked(ctx, saved, peers)
	}

	var key wgtypes.Key
	if validated.Enabled {
		key, err = loadOrCreatePrivateKey(s.keyPath)
		if err != nil {
			s.lastReason = "remote_key_unavailable"
			return Status{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}
	if s.runtime != nil {
		s.runtime.close()
		s.runtime = nil
	}

	if validated.Enabled {
		if err := s.startLocked(validated, key, peers); err != nil {
			s.lastReason = "remote_listen_failed"
			if old.Enabled {
				s.restoreLocked(old, key, peers)
			}
			return Status{}, err
		}
		s.startRenewalLocked(validated)
	} else {
		s.stopRenewalLocked()
	}
	if err := s.store.saveConfig(ctx, validated); err != nil {
		if s.runtime != nil {
			s.runtime.close()
			s.runtime = nil
		}
		if validated.Enabled && old.Enabled {
			s.restoreLocked(old, key, peers)
		}
		return Status{}, err
	}
	if !validated.Enabled {
		s.lastReason = ""
		s.endpointChanged = false
	}
	saved, err := s.store.config(ctx)
	if err != nil {
		return Status{}, err
	}
	return s.statusLocked(ctx, saved, peers)
}

func (s *Service) CreatePeer(ctx context.Context, name string) (Provision, error) {
	name, err := validatePeerName(name)
	if err != nil {
		return Provision{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.store.config(ctx)
	if err != nil {
		return Provision{}, err
	}
	if !cfg.Enabled {
		return Provision{}, ErrDisabled
	}
	if s.runtime == nil {
		return Provision{}, ErrNotReady
	}
	serverKey, err := readPrivateKey(s.keyPath)
	if err != nil {
		return Provision{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	clientKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return Provision{}, fmt.Errorf("%w: generating client key: %v", ErrUnavailable, err)
	}
	peer, err := s.store.createPeer(ctx, name, clientKey.PublicKey().String())
	if err != nil {
		return Provision{}, err
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		_ = s.store.discardPeer(ctx, peer.ID)
		return Provision{}, err
	}
	if err := s.runtime.applyPeers(peers); err != nil {
		_ = s.store.discardPeer(ctx, peer.ID)
		s.recoverRuntimeLocked(ctx, cfg)
		return Provision{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	client := clientConfig(clientKey, peer.Address, serverKey.PublicKey(), cfg.Endpoint)
	qr, err := discovery.QRSVG(client)
	if err != nil {
		_ = s.store.revokePeer(ctx, peer.ID)
		s.recoverRuntimeLocked(ctx, cfg)
		return Provision{}, fmt.Errorf("%w: rendering client QR: %v", ErrUnavailable, err)
	}
	return Provision{
		Peer:         peer,
		ClientConfig: client,
		QRSVG:        qr,
		TunnelURL:    tunnelURL(s.httpPort),
	}, nil
}

func (s *Service) RevokePeer(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.revokePeer(ctx, id); err != nil {
		return err
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		_ = s.store.undoRevoke(ctx, id)
		return err
	}
	if s.runtime == nil {
		return nil // persisted revocation is enough while the tunnel is down
	}
	if err := s.runtime.applyPeers(peers); err != nil {
		// The peer is persistently revoked. Closing the entire tunnel guarantees
		// the current process cannot keep an old session alive.
		s.runtime.close()
		s.runtime = nil
		s.lastReason = "remote_peer_reload_failed"
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.store.config(ctx)
	if err != nil {
		return Status{}, err
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		return Status{}, err
	}
	return s.statusLocked(ctx, cfg, peers)
}

func (s *Service) statusLocked(_ context.Context, cfg Config, peers []Peer) (Status, error) {
	status := Status{
		Config:       cfg,
		State:        "disabled",
		TunnelURL:    tunnelURL(s.httpPort),
		Reachability: "unverified",
		Peers:        peers,
	}
	if key, err := readPrivateKey(s.keyPath); err == nil {
		status.ServerPublicKey = key.PublicKey().String()
	}
	if cfg.Enabled {
		status.State = "error"
		status.Reason = s.lastReason
		if s.runtime != nil && s.lastReason == "" {
			status.State = "running"
			status.Peers = s.runtime.withStats(peers)
		}
	}

	// A tunnel that is running and a router that declined are two separate
	// facts, and collapsing them would produce the worst message of the two: a
	// listener perfectly up, said to be broken, or a connection nothing outside
	// can reach, said to be fine.
	status.DiscoveryReason = s.discoveryReason
	status.EndpointChanged = s.endpointChanged
	for _, peer := range status.Peers {
		if peer.LastHandshakeAt > 0 {
			status.Reachability = "confirmed"
			break
		}
	}
	return status, nil
}

func (s *Service) startLocked(cfg Config, key wgtypes.Key, peers []Peer) error {
	runtime, err := startTunnel(cfg, key, peers, s.httpPort, s.handler, s.log, func(err error) {
		s.log.Error("remote access listener stopped", "error", err)
		s.mu.Lock()
		s.lastReason = "remote_listener_stopped"
		s.mu.Unlock()
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	s.runtime = runtime
	s.lastReason = ""
	return nil
}

func (s *Service) restoreLocked(cfg Config, key wgtypes.Key, peers []Peer) {
	if !cfg.Enabled {
		return
	}
	if err := s.startLocked(cfg, key, peers); err != nil {
		s.lastReason = "remote_restore_failed"
		s.log.Error("the previous remote listener could not be restored", "error", err)
	}
}

func (s *Service) recoverRuntimeLocked(ctx context.Context, cfg Config) {
	if s.runtime != nil {
		s.runtime.close()
		s.runtime = nil
	}
	peers, err := s.store.peers(ctx)
	if err != nil {
		s.lastReason = "remote_peer_reload_failed"
		return
	}
	key, err := readPrivateKey(s.keyPath)
	if err != nil || s.startLocked(cfg, key, peers) != nil {
		s.lastReason = "remote_peer_reload_failed"
	}
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The renewal loop stops; the mapping deliberately stays. Somebody who
	// closes Theia for the night has not asked their router to forget the
	// forwarding -- only Disable means that, and the next start re-checks the
	// address anyway.
	s.stopRenewalLocked()
	if s.runtime != nil {
		s.runtime.close()
		s.runtime = nil
	}
}
