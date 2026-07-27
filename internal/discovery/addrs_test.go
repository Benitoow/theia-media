package discovery

import (
	"net"
	"testing"
)

func TestPreferredFrom(t *testing.T) {
	ips := func(list ...string) []net.IP {
		out := make([]net.IP, 0, len(list))
		for _, s := range list {
			ip := net.ParseIP(s)
			if ip == nil {
				t.Fatalf("bad test address %q", s)
			}
			out = append(out, ip)
		}
		return out
	}

	tests := []struct {
		name  string
		input []net.IP
		want  string // empty means "expect an error"
	}{
		{
			name:  "single private address",
			input: ips("192.168.1.19"),
			want:  "192.168.1.19",
		},
		{
			name: "private address wins over a public one listed first",
			// A machine with a public IPv4 still wants to hand out its LAN
			// address, not the one the rest of the internet sees.
			input: ips("203.0.113.7", "10.0.0.5"),
			want:  "10.0.0.5",
		},
		{
			name: "IPv6 is skipped",
			// The address is printed at startup and encoded in the QR code, so
			// it has to be something a human could retype.
			input: ips("fe80::1", "2001:db8::1", "172.16.3.4"),
			want:  "172.16.3.4",
		},
		{
			name: "link-local IPv4 is never chosen",
			// 169.254.x.x means DHCP failed; that address reaches nobody.
			input: ips("169.254.10.20"),
			want:  "",
		},
		{
			name:  "link-local is skipped in favour of a real address",
			input: ips("169.254.10.20", "192.168.0.42"),
			want:  "192.168.0.42",
		},
		{
			name:  "public address is used when nothing better exists",
			input: ips("203.0.113.7"),
			want:  "203.0.113.7",
		},
		{
			name:  "no addresses at all",
			input: nil,
			want:  "",
		},
		{
			name:  "only IPv6",
			input: ips("2001:db8::1"),
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := preferredFrom(tt.input)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("preferredFrom(%v) = %v, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("preferredFrom(%v) returned an unexpected error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("preferredFrom(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
