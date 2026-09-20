package discovery

import (
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

// The player finds its server by browsing for ServiceType, so an announcement
// is only correct if a browser actually finds it. This test runs the real
// responder and the real query, and it is deliberately written the way the
// player will use the protocol rather than against the library's internals:
// a rename of ServiceType would otherwise pass every unit test and leave the
// player unable to see any server at all.
//
// It skips when this machine cannot carry multicast, which is a real state and
// not a hypothetical one: a development machine here delivers no mDNS traffic
// at all, so the announcement can be perfectly correct and still be invisible.
// The skip is decided by a loopback probe rather than by guessing, so the test
// runs wherever mDNS works.
func TestAnnouncementIsDiscoverable(t *testing.T) {
	if v4, v6 := multicastFamilies(); !v4 && !v6 {
		t.Skip("no multicast family is available on this machine")
	}
	if !multicastLoopbackWorks(t) {
		t.Skip("multicast is bound but does not loop back on this machine; mDNS cannot be verified here")
	}

	// A distinct instance name, so a real Theia already running on this machine
	// can never be mistaken for the one under test.
	const hostname = "theia-mdns-test"
	const version = "3.3.1-test"
	port := freePort(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	announcer, err := Announce(hostname, port, version, log)
	if err != nil {
		t.Fatalf("Announce: %v", err)
	}
	defer announcer.Close()

	// The responder answers asynchronously, so a browse started in the same
	// instant can miss the first reply. The query is retried rather than run
	// once, which is also what a client does.
	deadline := time.Now().Add(4 * time.Second)
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		entries := make(chan *mdns.ServiceEntry, 8)
		go func() {
			_ = mdns.Query(&mdns.QueryParam{
				Service: ServiceType,
				Domain:  "local",
				Timeout: time.Second,
				Entries: entries,
			})
			close(entries)
		}()

		for entry := range entries {
			if entry.Host != hostname+".local." || entry.Port != port {
				continue
			}
			// The TXT records are what a client shows while it connects and
			// what makes a version mismatch visible, so finding the service
			// without them is not a pass.
			txt := strings.Join(entry.InfoFields, ",")
			if !strings.Contains(txt, txtNameKey+"=Theia") {
				t.Fatalf("found the service without its name record: %q", entry.InfoFields)
			}
			if !strings.Contains(txt, txtVersionKey+"="+version) {
				t.Fatalf("found the service without its version record: %q", entry.InfoFields)
			}
			if entry.AddrV4 == nil && entry.AddrV6 == nil {
				t.Fatalf("the service was announced without any address")
			}
			return
		}
		t.Logf("attempt %d found nothing yet", attempt)
	}
	t.Fatalf("browsing for %s never found %s on port %d", ServiceType, hostname, port)
}

// freePort asks the kernel for a port nobody is using, so the test cannot
// collide with a real Theia or with another test run.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// multicastLoopbackWorks reports whether a datagram sent to the mDNS group is
// delivered back to a listener on the same machine.
//
// It is the weakest question that still means anything: if this fails, no mDNS
// exchange can succeed here whatever the code does, and reporting the
// announcement as broken would be blaming the package for the network.
// IP_MULTICAST_LOOP is on by default, so a working stack returns the packet to
// the sender.
func multicastLoopbackWorks(t *testing.T) bool {
	t.Helper()

	conn, err := net.ListenMulticastUDP("udp4", nil, &net.UDPAddr{IP: mdnsIPv4, Port: mdnsPort})
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(1 << 12)

	writer, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: mdnsIPv4, Port: mdnsPort})
	if err != nil {
		return false
	}
	defer writer.Close()

	// A minimal, well-formed mDNS header: twelve bytes, no questions. Nothing
	// parses it here - only delivery is under test.
	probe := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	buf := make([]byte, 2048)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := writer.Write(probe); err != nil {
			return false
		}
		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		if _, _, err := conn.ReadFromUDP(buf); err == nil {
			return true
		}
	}
	return false
}
