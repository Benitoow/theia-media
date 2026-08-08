package portmap

import (
	"context"
	"encoding/binary"

	"fmt"
	"net"
	"net/netip"
	"time"
)

// NAT-PMP (RFC 6886) in about a hundred lines, because that is genuinely all it
// is: two datagrams to the gateway on UDP 5351, twelve and sixteen bytes back.
//
// It is the second choice rather than the first because UPnP is what French
// ISP boxes ship, but it costs little and covers the routers that answer
// nothing else -- AirPort, OpenWrt with the daemon on, a few ISP firmwares.

const (
	natpmpPort     = 5351
	natpmpVersion  = 0
	opExternalAddr = 0
	opMapUDP       = 1

	// Long enough that a renewal cycle is comfortable, short enough that a
	// mapping left behind by a crash expires the same afternoon.
	natpmpLifetime = 2 * time.Hour
)

func discoverNATPMP(ctx context.Context, internalPort int) (Mapping, error) {
	var last error
	for _, gateway := range likelyGateways() {
		mapping, err := natpmpAt(ctx, gateway, internalPort, natpmpLifetime)
		if err == nil {
			return mapping, nil
		}
		last = moreSpecific(last, err)
		if ctx.Err() != nil {
			break
		}
	}
	if last == nil {
		last = ErrNoGateway
	}
	return Mapping{}, last
}

func natpmpAt(ctx context.Context, gateway netip.Addr, internalPort int, lifetime time.Duration) (Mapping, error) {
	conn, err := net.Dial("udp4", net.JoinHostPort(gateway.String(), fmt.Sprint(natpmpPort)))
	if err != nil {
		return Mapping{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	// RFC 6886 asks for exponential backoff starting at 250 ms. Two tries is
	// the useful part of that: a router either speaks this or does not, and a
	// person is waiting.
	_ = conn.SetDeadline(time.Now().Add(1200 * time.Millisecond))

	external, err := natpmpExternalAddress(conn)
	if err != nil {
		return Mapping{}, err
	}
	if err := checkPublic(external); err != nil {
		return Mapping{}, err
	}

	_ = conn.SetDeadline(time.Now().Add(1500 * time.Millisecond))
	externalPort, granted, err := natpmpMap(conn, internalPort, lifetime)
	if err != nil {
		return Mapping{}, err
	}

	return Mapping{
		Method:       "natpmp",
		ExternalIP:   external,
		ExternalPort: externalPort,
		InternalPort: internalPort,
		Lifetime:     granted,
	}, nil
}

func natpmpExternalAddress(conn net.Conn) (netip.Addr, error) {
	if _, err := conn.Write([]byte{natpmpVersion, opExternalAddr}); err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	reply := make([]byte, 16)
	n, err := conn.Read(reply)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	if n < 12 || reply[1] != opExternalAddr+128 {
		return netip.Addr{}, fmt.Errorf("%w: unexpected reply", ErrNoGateway)
	}
	if code := binary.BigEndian.Uint16(reply[2:4]); code != 0 {
		return netip.Addr{}, fmt.Errorf("%w: result code %d", ErrRefused, code)
	}
	return netip.AddrFrom4([4]byte{reply[8], reply[9], reply[10], reply[11]}), nil
}

func natpmpMap(conn net.Conn, internalPort int, lifetime time.Duration) (int, time.Duration, error) {
	request := make([]byte, 12)
	request[0] = natpmpVersion
	request[1] = opMapUDP
	binary.BigEndian.PutUint16(request[4:6], uint16(internalPort))
	// Suggesting the same number keeps the endpoint predictable across renewals
	// and reboots. The router is free to answer with a different one.
	binary.BigEndian.PutUint16(request[6:8], uint16(internalPort))
	binary.BigEndian.PutUint32(request[8:12], uint32(lifetime.Seconds()))

	if _, err := conn.Write(request); err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	reply := make([]byte, 16)
	n, err := conn.Read(reply)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	if n < 16 || reply[1] != opMapUDP+128 {
		return 0, 0, fmt.Errorf("%w: unexpected reply", ErrNoGateway)
	}
	if code := binary.BigEndian.Uint16(reply[2:4]); code != 0 {
		return 0, 0, fmt.Errorf("%w: result code %d", ErrRefused, code)
	}
	external := int(binary.BigEndian.Uint16(reply[10:12]))
	granted := time.Duration(binary.BigEndian.Uint32(reply[12:16])) * time.Second
	if external == 0 {
		return 0, 0, fmt.Errorf("%w: the router granted port zero", ErrRefused)
	}
	return external, granted, nil
}

func releaseNATPMP(ctx context.Context, mapping Mapping) {
	for _, gateway := range likelyGateways() {
		conn, err := net.Dial("udp4", net.JoinHostPort(gateway.String(), fmt.Sprint(natpmpPort)))
		if err != nil {
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))
		// A lifetime of zero is how RFC 6886 spells "delete". The suggested
		// external port must be zero too.
		request := make([]byte, 12)
		request[1] = opMapUDP
		binary.BigEndian.PutUint16(request[4:6], uint16(mapping.InternalPort))
		_, _ = conn.Write(request)
		_, _ = conn.Read(make([]byte, 16))
		conn.Close()
		if ctx.Err() != nil {
			return
		}
	}
}

// likelyGateways guesses where the router is.
//
// Reading the routing table means one syscall path per operating system, and
// getting it wrong on any of them means remote access quietly not working
// there. The guess is cheap and covers what home networks actually look like:
// the router is the first or the last usable address of the subnet this
// machine is on. Freeboxes sit at .254, almost everything else at .1.
//
// A wrong guess costs one datagram that goes unanswered. UPnP, which needs no
// guess at all, is running at the same time.
func likelyGateways() []netip.Addr {
	seen := map[netip.Addr]bool{}
	var candidates []netip.Addr

	interfaces, err := net.Interfaces()
	if err != nil {
		return candidates
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, raw := range addrs {
			prefix, ok := raw.(*net.IPNet)
			if !ok || prefix.IP.To4() == nil {
				continue
			}
			addr, ok := netip.AddrFromSlice(prefix.IP.To4())
			if !ok || !addr.IsPrivate() {
				continue
			}
			ones, bits := prefix.Mask.Size()
			if bits != 32 || ones < 16 {
				continue
			}
			network := addr.As4()
			mask := prefix.Mask
			for i := range network {
				network[i] &= mask[i]
			}
			for _, offset := range []byte{1, 254} {
				guess := network
				guess[3] = offset
				candidate := netip.AddrFrom4(guess)
				if candidate == addr || seen[candidate] {
					continue
				}
				seen[candidate] = true
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}
