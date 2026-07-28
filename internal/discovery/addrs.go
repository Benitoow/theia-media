package discovery

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// Address is one way to reach this machine, with enough context to rank it.
type Address struct {
	IP        net.IP `json:"-"`
	Text      string `json:"address"`
	Interface string `json:"interface"`

	// Virtual marks an adapter created by a hypervisor, container runtime or
	// VPN. Those carry perfectly valid private addresses that no phone on the
	// Wi-Fi can reach.
	Virtual bool `json:"virtual"`
}

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

// virtualAdapterNames are substrings that mark an interface as belonging to a
// hypervisor, container runtime or tunnel rather than to a real network.
//
// This matters more than it looks. A machine running WSL, Docker or Hyper-V has
// several private IPv4 addresses, all equally valid to the naive check, and
// only one of them is reachable from a phone on the sofa. A QR code pointing at
// a Hyper-V switch is worse than no QR code at all: it fails silently and looks
// like the server is broken.
var virtualAdapterNames = []string{
	"vethernet", "vmware", "virtualbox", "vbox", "hyper-v", "docker", "wsl",
	"tap-", "tap ", "tun", "loopback", "bluetooth", "vpn", "tailscale",
	"zerotier", "wireguard", "utun", "bridge", "veth", "npcap",
	// Docker names its user-defined bridges br-<hash>. They carry private
	// addresses that nothing outside the host can reach.
	"br-",
}

func looksVirtual(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range virtualAdapterNames {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// Candidates returns the IPv4 addresses another device could dial, best first.
//
// There is no portable way in pure Go to ask which interface carries the
// default route, so this ranks by what can be observed: real adapters before
// virtual ones, and private addresses before anything else. The interface name
// travels with each entry so the interface can offer the others when the first
// guess is wrong.
func Candidates() ([]Address, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing network interfaces: %w", err)
	}

	var out []Address
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			// Link-local means a DHCP lease that never arrived. That address
			// reaches nobody, so it is not a candidate at all.
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, Address{
				IP:        v4,
				Text:      v4.String(),
				Interface: iface.Name,
				Virtual:   looksVirtual(iface.Name),
			})
		}
	}

	sortCandidates(out)
	return out, nil
}

// sortCandidates ranks addresses best-first: real adapters before virtual ones,
// then private addresses before public.
//
// Separated from the enumeration so the ordering -- the thing the QR code
// actually depends on -- can be tested without depending on whatever interfaces
// the test machine happens to have.
func sortCandidates(list []Address) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Virtual != list[j].Virtual {
			return !list[i].Virtual
		}
		iPrivate, jPrivate := list[i].IP.IsPrivate(), list[j].IP.IsPrivate()
		if iPrivate != jPrivate {
			return iPrivate
		}
		return false
	})
}

// PreferredAddr returns the IPv4 address most likely to be the one other
// devices on the network should dial. It is what gets printed at startup and
// what the onboarding QR code encodes.
func PreferredAddr() (net.IP, error) {
	candidates, err := Candidates()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no usable IPv4 address found on any network interface")
	}
	return candidates[0].IP, nil
}
