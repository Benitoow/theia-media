// Package portmap asks the local router to forward one UDP port and to say
// which address the internet sees.
//
// This is what turns remote access from a page of instructions into a switch.
// Before it, somebody had to find their router's admin page, forward a UDP
// port by hand, look up their public address somewhere, and type it back into
// Theia -- four steps, three of which are on somebody else's software.
//
// It changes nothing about the founding constraints, and that is the point.
// Both protocols speak only to the gateway on the local network: SSDP is a
// multicast to 239.255.255.250, NAT-PMP a datagram to the default gateway.
// Neither contacts a relay, a control plane, a STUN server or an
// endpoint-discovery service, so decision 43 stands and the only internet calls
// Theia makes are still TMDB and GitHub Releases. The address comes back from
// the router, which knows it because it holds it.
//
// It also refuses to lie. A router behind carrier-grade NAT will happily report
// its own 100.64.0.0/10 address, and a generated client configuration pointing
// at one cannot work. That is reported as the specific thing it is rather than
// left to fail as a timeout the evening somebody is away from home.
package portmap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"
)

var (
	// ErrNoGateway means nothing on the network answered. Neither protocol is
	// mandatory, and plenty of routers ship with both switched off.
	ErrNoGateway = errors.New("portmap: no router answered")

	// ErrRefused means a router answered and declined to map the port.
	ErrRefused = errors.New("portmap: the router refused the mapping")

	// ErrNotPublic means the mapping worked but the address behind it is not
	// reachable from the internet -- carrier-grade NAT, or a second router in
	// front of this one.
	ErrNotPublic = errors.New("portmap: the router's own address is not public")
)

// Mapping is a forwarded port and the address it is reachable at.
type Mapping struct {
	// Method is "upnp" or "natpmp", reported so a support question has an
	// answer and so the interface can say which one worked.
	Method string

	ExternalIP   netip.Addr
	ExternalPort int
	InternalPort int

	// Lifetime is zero for a mapping the router keeps until it is deleted.
	Lifetime time.Duration
}

// Endpoint is the host:port form a WireGuard client configuration wants.
func (m Mapping) Endpoint() string {
	return net.JoinHostPort(m.ExternalIP.String(), fmt.Sprint(m.ExternalPort))
}

// Discover asks for a UDP mapping, trying both protocols at once.
//
// Concurrently rather than in sequence because they fail by timing out: a
// router that speaks neither would otherwise cost the sum of two waits with
// somebody watching a spinner. The first success wins; if both fail, the more
// specific failure is returned, since "your connection is behind carrier-grade
// NAT" is worth saying and "nothing answered" is not worth saying twice.
func Discover(ctx context.Context, internalPort int, description string) (Mapping, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	type attempt struct {
		mapping Mapping
		err     error
	}
	results := make(chan attempt, 2)

	go func() {
		mapping, err := discoverUPnP(ctx, internalPort, description)
		results <- attempt{mapping, err}
	}()
	go func() {
		mapping, err := discoverNATPMP(ctx, internalPort)
		results <- attempt{mapping, err}
	}()

	var worst error
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err == nil {
			return got.mapping, nil
		}
		worst = moreSpecific(worst, got.err)
	}
	if worst == nil {
		worst = ErrNoGateway
	}
	return Mapping{}, worst
}

// Release withdraws a mapping. Best effort by design: a router that has been
// rebooted, replaced or simply forgotten the mapping is not an error worth
// showing somebody who has just switched remote access off.
func Release(ctx context.Context, mapping Mapping) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	switch mapping.Method {
	case "upnp":
		releaseUPnP(ctx, mapping)
	case "natpmp":
		releaseNATPMP(ctx, mapping)
	}
}

// moreSpecific keeps the error that tells somebody something they can act on.
func moreSpecific(current, next error) error {
	rank := func(err error) int {
		switch {
		case errors.Is(err, ErrNotPublic):
			return 3
		case errors.Is(err, ErrRefused):
			return 2
		case err != nil:
			return 1
		default:
			return 0
		}
	}
	if rank(next) > rank(current) {
		return next
	}
	return current
}

// checkPublic is the honesty gate both protocols pass through.
func checkPublic(addr netip.Addr) error {
	if !addr.IsValid() || addr.IsUnspecified() {
		return fmt.Errorf("%w: the router reported no address", ErrNoGateway)
	}
	if isPrivate(addr) {
		return fmt.Errorf("%w: %s", ErrNotPublic, addr)
	}
	return nil
}

// isPrivate covers the ranges a router can report while still not being on the
// internet. 100.64.0.0/10 is the important one and the least obvious: it is
// carrier-grade NAT, where the address belongs to the ISP and thousands of
// subscribers sit behind it. No port forwarding reaches through it.
func isPrivate(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsPrivate() ||
		addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	if addr.Is4() {
		b := addr.As4()
		// 100.64.0.0/10, carrier-grade NAT (RFC 6598).
		if b[0] == 100 && b[1] >= 64 && b[1] <= 127 {
			return true
		}
		// 192.0.0.0/24 and 198.18.0.0/15, both reserved rather than routable.
		if b[0] == 192 && b[1] == 0 && b[2] == 0 {
			return true
		}
		if b[0] == 198 && (b[1] == 18 || b[1] == 19) {
			return true
		}
	}
	return false
}

// localAddressToward finds the LAN address this machine would use to reach the
// gateway, which is what a port mapping has to point at.
//
// A UDP "connection" sends nothing; it only makes the kernel run its routing
// table and pick a source address. That is more reliable than walking the
// interface list, which on a machine with a VPN, WSL, Docker and a virtual
// switch offers half a dozen equally plausible private addresses.
func localAddressToward(gateway netip.Addr) (netip.Addr, error) {
	conn, err := net.Dial("udp4", net.JoinHostPort(gateway.String(), "9"))
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, errors.New("portmap: unexpected local address type")
	}
	addr, ok := netip.AddrFromSlice(local.IP)
	if !ok {
		return netip.Addr{}, errors.New("portmap: unreadable local address")
	}
	return addr.Unmap(), nil
}
