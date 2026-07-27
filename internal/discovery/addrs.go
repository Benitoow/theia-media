package discovery

import (
	"fmt"
	"net"
)

// LANAddrs returns every non-loopback unicast address on an interface that is
// up. These are the addresses advertised over mDNS, so both IPv4 and IPv6 are
// included -- a device that only speaks one of the two still finds the server.
func LANAddrs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing network interfaces: %w", err)
	}

	var ips []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue // an interface disappearing mid-enumeration is not fatal
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			ips = append(ips, ipnet.IP)
		}
	}
	return ips, nil
}

// PreferredAddr returns the IPv4 address most likely to be the one other
// devices on the network should dial. It is what gets printed at startup and,
// from M6 on, what the onboarding QR code encodes.
func PreferredAddr() (net.IP, error) {
	ips, err := LANAddrs()
	if err != nil {
		return nil, err
	}
	return preferredFrom(ips)
}

// preferredFrom holds the actual selection rule, separated from the interface
// enumeration so it can be tested without depending on the host's network.
//
// Preference order: a private RFC 1918 address -- the normal home-network case
// -- beats any other IPv4. Link-local 169.254.x.x is skipped outright, because
// an address handed out by a failed DHCP lease reaches nobody. IPv6 is ignored
// here: this address ends up in a URL a human may have to read off a screen.
func preferredFrom(ips []net.IP) (net.IP, error) {
	var fallback net.IP
	for _, ip := range ips {
		v4 := ip.To4()
		if v4 == nil || v4.IsLinkLocalUnicast() {
			continue
		}
		if v4.IsPrivate() {
			return v4, nil
		}
		if fallback == nil {
			fallback = v4
		}
	}
	if fallback == nil {
		return nil, fmt.Errorf("no usable IPv4 address found on any network interface")
	}
	return fallback, nil
}
