// Package discovery makes Theia findable on the local network without anyone
// having to type an IP address.
//
// Two mechanisms, deliberately unequal in status. The mDNS announcement here
// lets clients that resolve .local names reach "theia.local" directly, and the
// address helpers in addrs.go feed the onboarding QR code. mDNS is the pleasant
// shortcut, not the contract: plenty of smart-TV browsers never resolve .local
// at all, so nothing in this package is allowed to be load-bearing or fatal.
package discovery

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/hashicorp/mdns"
)

// The multicast group and port mDNS is spoken on, per RFC 6762.
var (
	mdnsIPv4 = net.IP{224, 0, 0, 251}
	mdnsIPv6 = net.ParseIP("ff02::fb")
)

const mdnsPort = 5353

// Announcer holds a running mDNS responder.
type Announcer struct {
	server *mdns.Server
}

// multicastFamilies reports which IP families can actually receive mDNS traffic
// on this machine.
//
// This exists because hashicorp/mdns discards both of its bind errors and only
// fails when *both* listeners are down. A responder that ended up answering on
// IPv6 alone is therefore indistinguishable from a healthy one, and we would
// log a cheerful success for something that half works. Probing first is the
// only way to say something true. The probe sockets are closed immediately;
// they use SO_REUSEADDR, so holding them briefly does not stop the library from
// binding the same addresses a moment later.
func multicastFamilies() (v4, v6 bool) {
	if c, err := net.ListenMulticastUDP("udp4", nil, &net.UDPAddr{IP: mdnsIPv4, Port: mdnsPort}); err == nil {
		c.Close()
		v4 = true
	}
	if c, err := net.ListenMulticastUDP("udp6", nil, &net.UDPAddr{IP: mdnsIPv6, Port: mdnsPort}); err == nil {
		c.Close()
		v6 = true
	}
	return v4, v6
}

// Announce starts answering mDNS queries for "<hostname>.local", pointing at
// this machine's LAN addresses on the given port.
//
// Callers are expected to treat a failure as a warning, not a fatal error.
// Another responder may already own UDP 5353 -- Bonjour ships with iTunes and
// several Adobe products on Windows -- and Theia is perfectly usable over a
// plain IP address without it.
func Announce(hostname string, port int, log *slog.Logger) (*Announcer, error) {
	ips, err := LANAddrs()
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no non-loopback network interface is up")
	}

	// The trailing dot matters twice over: hashicorp/mdns validates this as a
	// fully-qualified name, and it is the exact string it matches incoming
	// A/AAAA questions against.
	fqdn := hostname + ".local."

	// Passing the addresses explicitly is not optional. Left empty, the library
	// falls back to net.LookupIP(fqdn) -- which asks the system resolver to look
	// up the very name we are about to start answering for, and fails.
	service, err := mdns.NewMDNSService(
		hostname,     // instance name, as it appears in service browsers
		"_http._tcp", // Theia is a web server; nothing more specific is warranted
		"",           // domain, defaults to "local."
		fqdn,
		port,
		ips,
		[]string{"Theia media server"},
	)
	if err != nil {
		return nil, fmt.Errorf("building mDNS service record: %w", err)
	}

	v4, v6 := multicastFamilies()
	if !v4 && !v6 {
		return nil, fmt.Errorf("cannot listen on UDP %d, another mDNS responder already owns it", mdnsPort)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("starting mDNS responder: %w", err)
	}

	log.Info("mDNS announcement started",
		"hostname", hostname+".local",
		"port", port,
		"addresses", len(ips),
		"ipv4", v4,
		"ipv6", v6,
	)
	if !v4 {
		// Almost every home network still finds servers over IPv4. Answering
		// only on IPv6 means most devices will never resolve theia.local, so
		// this deserves to be visible rather than buried in the info line.
		log.Warn("mDNS is answering on IPv6 only, most devices will not resolve " +
			hostname + ".local; use the IP address shown above instead")
	}
	return &Announcer{server: server}, nil
}

// Close stops the mDNS responder. It tolerates a nil receiver so that callers
// can defer it directly after an Announce that failed.
func (a *Announcer) Close() error {
	if a == nil || a.server == nil {
		return nil
	}
	return a.server.Shutdown()
}
