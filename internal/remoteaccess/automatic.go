package remoteaccess

import (
	"context"
	"errors"
	"time"

	"github.com/Benitoow/theia-media/internal/portmap"
)

// The automatic half of remote access: ask the router, do not ask the owner.
//
// What was here before was a UDP port field, a public endpoint field and a
// paragraph of instructions for forwarding a port on a router Theia has never
// seen. Both of those facts belong to the gateway, which already knows them,
// and both protocols that ask it -- UPnP IGD and NAT-PMP -- speak only to the
// local network. So nothing about decision 43 changes: there is still no relay,
// no control plane, and no endpoint-discovery service. The router is not the
// internet; it is the thing standing between this machine and it.
//
// The manual fields survive as the way through when a router says no, which is
// a real case: forwarding is switched off by default on some ISP firmware, and
// carrier-grade NAT cannot be forwarded through at all.

// mapPortLocked asks the router for a mapping and returns the endpoint it
// yields. The caller holds s.mu.
//
// A failure is never fatal on its own. If the owner has typed an endpoint by
// hand, that endpoint stands and the reason is recorded for the panel to
// explain; only a configuration with nothing to fall back on fails.
func (s *Service) mapPortLocked(ctx context.Context, cfg Config) (Config, string) {
	mapping, err := portmap.Discover(ctx, cfg.ListenPort, "Theia")
	if err != nil {
		s.log.Info("the router did not provide a port mapping",
			"port", cfg.ListenPort, "error", err)
		cfg.MappedMethod = ""
		cfg.MappedPort = 0
		return cfg, discoveryReason(err)
	}

	s.log.Info("remote access mapped a port through the router",
		"method", mapping.Method,
		"endpoint", mapping.Endpoint(),
		"lifetime", mapping.Lifetime)

	cfg.Endpoint = mapping.Endpoint()
	cfg.MappedMethod = mapping.Method
	cfg.MappedPort = mapping.ExternalPort
	s.mapping = mapping
	s.mapped = true
	return cfg, ""
}

// releaseMappingLocked withdraws the forwarding on the way out.
//
// Leaving a hole open in somebody's router after they switched the feature off
// would be the kind of thing this project exists not to do. Best effort: a
// router that has since been rebooted has already forgotten it.
func (s *Service) releaseMappingLocked() {
	if !s.mapped {
		return
	}
	mapping := s.mapping
	s.mapped = false
	s.mapping = portmap.Mapping{}
	go portmap.Release(context.Background(), mapping)
}

func discoveryReason(err error) string {
	switch {
	case errors.Is(err, portmap.ErrNotPublic):
		return DiscoveryNotPublic
	case errors.Is(err, portmap.ErrRefused):
		return DiscoveryRefused
	default:
		return DiscoveryNoGateway
	}
}

// startRenewalLocked keeps a leased mapping alive.
//
// Routers that only grant timed leases forget the forwarding after an hour,
// which would turn remote access into something that works this evening and
// not tomorrow -- the worst kind of failure, because nobody is there to see it
// happen. A mapping granted forever needs none of this and gets no goroutine.
func (s *Service) startRenewalLocked(cfg Config) {
	s.stopRenewalLocked()
	if !cfg.Automatic || !s.mapped || s.mapping.Lifetime <= 0 {
		return
	}

	interval := s.mapping.Lifetime / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.renewCancel = cancel

	go func(port int) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mapping, err := portmap.Discover(ctx, port, "Theia")
				if err != nil {
					s.log.Warn("renewing the router port mapping failed", "error", err)
					continue
				}
				s.adoptRenewal(ctx, mapping)
			}
		}
	}(cfg.ListenPort)
}

// adoptRenewal records a renewed mapping, and notices when the public address
// has changed underneath it.
//
// A domestic connection is renumbered on a reboot or a lease expiry. Existing
// client configurations then point at somebody else's address, which is worth
// knowing about rather than discovering from a phone in a hotel.
func (s *Service) adoptRenewal(ctx context.Context, mapping portmap.Mapping) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.mapping
	s.mapping = mapping
	s.mapped = true
	if previous.Endpoint() == mapping.Endpoint() {
		return
	}

	cfg, err := s.store.config(ctx)
	if err != nil || !cfg.Enabled || !cfg.Automatic {
		return
	}
	cfg.Endpoint = mapping.Endpoint()
	cfg.MappedMethod = mapping.Method
	cfg.MappedPort = mapping.ExternalPort
	if err := s.store.saveConfig(ctx, cfg); err != nil {
		s.log.Warn("recording a changed public address failed", "error", err)
		return
	}
	s.log.Warn("the public address changed; devices provisioned before now need a new configuration",
		"endpoint", mapping.Endpoint())
	s.endpointChanged = true
}

func (s *Service) stopRenewalLocked() {
	if s.renewCancel != nil {
		s.renewCancel()
		s.renewCancel = nil
	}
}
