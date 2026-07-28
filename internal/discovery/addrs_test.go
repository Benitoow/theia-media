package discovery

import (
	"net"
	"testing"
)

// addr builds a candidate the way Candidates would.
func addr(ip, iface string) Address {
	parsed := net.ParseIP(ip).To4()
	if parsed == nil {
		panic("bad test address " + ip)
	}
	return Address{IP: parsed, Text: ip, Interface: iface, Virtual: looksVirtual(iface)}
}

func TestSortCandidates(t *testing.T) {
	tests := []struct {
		name  string
		input []Address
		want  string // the address expected to rank first
	}{
		{
			name:  "a single private address",
			input: []Address{addr("192.168.1.19", "Wi-Fi")},
			want:  "192.168.1.19",
		},
		{
			name: "private beats public",
			// A machine with a public address still wants to hand out its LAN
			// one; the phone on the sofa cannot use the other.
			input: []Address{addr("203.0.113.7", "Ethernet"), addr("10.0.0.5", "Ethernet 2")},
			want:  "10.0.0.5",
		},
		{
			name: "a real adapter beats a virtual one",
			// The failure this ordering exists to prevent: a QR code pointing
			// at a Hyper-V switch scans fine and reaches nothing.
			input: []Address{
				addr("172.17.0.1", "docker0"),
				addr("192.168.1.19", "Wi-Fi"),
			},
			want: "192.168.1.19",
		},
		{
			name: "a real adapter wins even when listed last",
			input: []Address{
				addr("192.168.56.1", "VirtualBox Host-Only Network"),
				addr("172.28.0.1", "vEthernet (WSL)"),
				addr("192.168.1.19", "Wi-Fi"),
			},
			want: "192.168.1.19",
		},
		{
			name:  "a virtual adapter is still offered when it is all there is",
			input: []Address{addr("172.17.0.1", "docker0")},
			want:  "172.17.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortCandidates(tt.input)
			if tt.input[0].Text != tt.want {
				t.Errorf("first = %s (%s), want %s",
					tt.input[0].Text, tt.input[0].Interface, tt.want)
			}
		})
	}
}

func TestLooksVirtual(t *testing.T) {
	virtual := []string{
		"vEthernet (Default Switch)", "VMware Network Adapter VMnet1",
		"docker0", "VirtualBox Host-Only Network", "Tailscale",
		"br-1a2b3c", "utun3", "WSL (Hyper-V firewall)",
	}
	for _, name := range virtual {
		if !looksVirtual(name) {
			t.Errorf("looksVirtual(%q) = false, want true", name)
		}
	}

	real := []string{"Wi-Fi", "Ethernet", "eth0", "wlan0", "en0", "Ethernet 2"}
	for _, name := range real {
		if looksVirtual(name) {
			t.Errorf("looksVirtual(%q) = true, want false", name)
		}
	}
}

func TestCandidatesNeverOffersSomethingUnreachable(t *testing.T) {
	// Not an assertion about this machine's network, only that enumeration
	// filters out addresses no other device could ever dial.
	got, err := Candidates()
	if err != nil {
		t.Fatalf("Candidates returned an unexpected error: %v", err)
	}
	for _, c := range got {
		switch {
		case c.IP.To4() == nil:
			t.Errorf("%s is not IPv4", c.Text)
		case c.IP.IsLinkLocalUnicast():
			// 169.254.x.x means a DHCP lease that never arrived.
			t.Errorf("%s is link-local and reaches nobody", c.Text)
		case c.IP.IsLoopback():
			t.Errorf("%s is loopback and reaches no other device", c.Text)
		}
	}
}

func TestPreferredAddrMatchesTheFirstCandidate(t *testing.T) {
	candidates, err := Candidates()
	if err != nil {
		t.Fatal(err)
	}
	preferred, err := PreferredAddr()
	if len(candidates) == 0 {
		if err == nil {
			t.Error("PreferredAddr succeeded with no candidates")
		}
		return
	}
	if err != nil {
		t.Fatalf("PreferredAddr returned an unexpected error: %v", err)
	}
	if !preferred.Equal(candidates[0].IP) {
		t.Errorf("PreferredAddr = %v, want the top candidate %v", preferred, candidates[0].IP)
	}
}
