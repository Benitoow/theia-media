package portmap

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// UPnP Internet Gateway Device: SSDP to find the router, one XML document to
// find its control endpoint, then SOAP.
//
// This is the path that works on the boxes people actually have -- Freebox,
// Livebox, SFR, most consumer routers -- which is why it runs first.
//
// Everything here is deliberately paranoid about where it will talk. An SSDP
// reply is an unauthenticated UDP datagram from anybody on the network, and it
// names a URL that this process is about to fetch. So the address is checked
// before the fetch, not after: only a private address on this machine's own
// network is followed. Otherwise a device on the LAN could point Theia at an
// arbitrary host, which is a request Theia would be making on its owner's
// behalf and cannot be allowed to be.

const ssdpAddress = "239.255.255.250:1900"

// wanServices are the two service types that can forward a port, newest first.
var wanServices = []string{
	"urn:schemas-upnp-org:service:WANIPConnection:2",
	"urn:schemas-upnp-org:service:WANIPConnection:1",
	"urn:schemas-upnp-org:service:WANPPPConnection:1",
}

type upnpTarget struct {
	controlURL  string
	serviceType string
	gateway     netip.Addr
	localIP     netip.Addr
}

func discoverUPnP(ctx context.Context, internalPort int, description string) (Mapping, error) {
	target, err := findGatewayService(ctx)
	if err != nil {
		return Mapping{}, err
	}

	external, err := upnpExternalAddress(ctx, target)
	if err != nil {
		return Mapping{}, err
	}
	if err := checkPublic(external); err != nil {
		return Mapping{}, err
	}

	// A lease of zero means "until deleted", which is what most consumer
	// firmware supports and what survives Theia being closed for a week. The
	// ones that refuse it say so with error 725, and get an hour instead --
	// renewed while the tunnel is up.
	lifetime := time.Duration(0)
	err = upnpAddMapping(ctx, target, internalPort, description, 0)
	if err != nil && strings.Contains(err.Error(), "725") {
		lifetime = time.Hour
		err = upnpAddMapping(ctx, target, internalPort, description, lifetime)
	}
	if err != nil {
		return Mapping{}, err
	}

	return Mapping{
		Method:       "upnp",
		ExternalIP:   external,
		ExternalPort: internalPort,
		InternalPort: internalPort,
		Lifetime:     lifetime,
	}, nil
}

// findGatewayService sends one M-SEARCH and follows the first answer that
// describes a service able to forward a port.
func findGatewayService(ctx context.Context) (upnpTarget, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return upnpTarget{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	defer conn.Close()

	multicast, err := net.ResolveUDPAddr("udp4", ssdpAddress)
	if err != nil {
		return upnpTarget{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}

	search := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: " + ssdpAddress + "\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n"

	deadline := time.Now().Add(4 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	// Twice, a moment apart: SSDP runs over UDP multicast and a single lost
	// datagram would otherwise read as "this network has no router".
	for i := 0; i < 2; i++ {
		if _, err := conn.WriteTo([]byte(search), multicast); err != nil {
			return upnpTarget{}, fmt.Errorf("%w: %v", ErrNoGateway, err)
		}
		time.Sleep(150 * time.Millisecond)
	}

	buffer := make([]byte, 2048)
	tried := map[string]bool{}
	for time.Now().Before(deadline) {
		n, from, err := conn.ReadFrom(buffer)
		if err != nil {
			break
		}
		location := headerValue(string(buffer[:n]), "LOCATION")
		if location == "" || tried[location] {
			continue
		}
		tried[location] = true

		sender, ok := netip.AddrFromSlice(from.(*net.UDPAddr).IP)
		if !ok {
			continue
		}
		target, err := describeGateway(ctx, location, sender.Unmap())
		if err != nil {
			continue
		}
		return target, nil
	}
	return upnpTarget{}, ErrNoGateway
}

// describeGateway fetches the device description and picks a control URL.
func describeGateway(ctx context.Context, location string, sender netip.Addr) (upnpTarget, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return upnpTarget{}, err
	}
	host, _, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		host = parsed.Host
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		// A name rather than an address means a DNS lookup this must not make
		// on the say-so of a multicast datagram.
		return upnpTarget{}, fmt.Errorf("%w: the description URL is not a numeric address", ErrNoGateway)
	}
	if !addr.IsPrivate() && !addr.IsLinkLocalUnicast() && !addr.IsLoopback() {
		return upnpTarget{}, fmt.Errorf("%w: the description URL leaves the local network", ErrNoGateway)
	}
	if addr != sender.Unmap() {
		// The device that answered and the host it points at must be the same
		// machine. Otherwise one device on the network can redirect this fetch.
		return upnpTarget{}, fmt.Errorf("%w: the description URL points elsewhere", ErrNoGateway)
	}

	body, err := fetch(ctx, location)
	if err != nil {
		return upnpTarget{}, err
	}

	var root struct {
		Device upnpDevice `xml:"device"`
	}
	if err := xml.Unmarshal(body, &root); err != nil {
		return upnpTarget{}, err
	}

	for _, wanted := range wanServices {
		if control, ok := findService(root.Device, wanted); ok {
			resolved, err := parsed.Parse(control)
			if err != nil {
				continue
			}
			if resolved.Hostname() != addr.String() {
				// A relative controlURL is normal; one that walks off to
				// another host is not.
				continue
			}
			local, err := localAddressToward(addr)
			if err != nil {
				return upnpTarget{}, err
			}
			return upnpTarget{
				controlURL:  resolved.String(),
				serviceType: wanted,
				gateway:     addr,
				localIP:     local,
			}, nil
		}
	}
	return upnpTarget{}, fmt.Errorf("%w: the router has no port-forwarding service", ErrRefused)
}

type upnpDevice struct {
	DeviceType  string       `xml:"deviceType"`
	ServiceList []upnpSvc    `xml:"serviceList>service"`
	Devices     []upnpDevice `xml:"deviceList>device"`
}

type upnpSvc struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// findService walks the nested device tree. The WAN connection service is two
// levels down on every gateway: device > WANDevice > WANConnectionDevice.
func findService(device upnpDevice, wanted string) (string, bool) {
	for _, service := range device.ServiceList {
		if strings.EqualFold(strings.TrimSpace(service.ServiceType), wanted) {
			return strings.TrimSpace(service.ControlURL), true
		}
	}
	for _, child := range device.Devices {
		if control, ok := findService(child, wanted); ok {
			return control, true
		}
	}
	return "", false
}

func upnpExternalAddress(ctx context.Context, target upnpTarget) (netip.Addr, error) {
	body, err := soap(ctx, target, "GetExternalIPAddress", "")
	if err != nil {
		return netip.Addr{}, err
	}
	value := elementText(body, "NewExternalIPAddress")
	addr, parseErr := netip.ParseAddr(strings.TrimSpace(value))
	if parseErr != nil {
		return netip.Addr{}, fmt.Errorf("%w: the router reported no usable address", ErrNoGateway)
	}
	return addr.Unmap(), nil
}

func upnpAddMapping(ctx context.Context, target upnpTarget, port int, description string, lease time.Duration) error {
	arguments := "" +
		"<NewRemoteHost></NewRemoteHost>" +
		"<NewExternalPort>" + strconv.Itoa(port) + "</NewExternalPort>" +
		"<NewProtocol>UDP</NewProtocol>" +
		"<NewInternalPort>" + strconv.Itoa(port) + "</NewInternalPort>" +
		"<NewInternalClient>" + target.localIP.String() + "</NewInternalClient>" +
		"<NewEnabled>1</NewEnabled>" +
		"<NewPortMappingDescription>" + escapeXML(description) + "</NewPortMappingDescription>" +
		"<NewLeaseDuration>" + strconv.Itoa(int(lease.Seconds())) + "</NewLeaseDuration>"

	if _, err := soap(ctx, target, "AddPortMapping", arguments); err != nil {
		return err
	}
	return nil
}

func releaseUPnP(ctx context.Context, mapping Mapping) {
	target, err := findGatewayService(ctx)
	if err != nil {
		return
	}
	arguments := "" +
		"<NewRemoteHost></NewRemoteHost>" +
		"<NewExternalPort>" + strconv.Itoa(mapping.ExternalPort) + "</NewExternalPort>" +
		"<NewProtocol>UDP</NewProtocol>"
	_, _ = soap(ctx, target, "DeletePortMapping", arguments)
}

func soap(ctx context.Context, target upnpTarget, action, arguments string) ([]byte, error) {
	envelope := `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>` +
		`<u:` + action + ` xmlns:u="` + target.serviceType + `">` + arguments +
		`</u:` + action + `></s:Body></s:Envelope>`

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.controlURL,
		strings.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	request.Header.Set("SOAPAction", `"`+target.serviceType+`#`+action+`"`)

	response, err := gatewayClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	if response.StatusCode != http.StatusOK {
		// The UPnP error code is more use than the HTTP status: 718 is "that
		// port is already forwarded to another machine", 725 "this router only
		// does permanent leases", 606 "not authorised".
		code := elementText(body, "errorCode")
		return nil, fmt.Errorf("%w: %s returned %d (upnp error %s)",
			ErrRefused, action, response.StatusCode, code)
	}
	return body, nil
}

// gatewayClient never follows a redirect and never keeps a connection: it is
// talking to one device on the local network, briefly, and a redirect would be
// that device sending Theia somewhere it has not checked.
var gatewayClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{DisableKeepAlives: true},
}

func fetch(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	response, err := gatewayClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoGateway, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: the description returned %d", ErrNoGateway, response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 512<<10))
}

// headerValue reads one header out of an SSDP reply, which is HTTP-shaped
// without being HTTP.
func headerValue(response, name string) string {
	scanner := bufio.NewScanner(strings.NewReader(response))
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(line[:colon]), name) {
			return strings.TrimSpace(line[colon+1:])
		}
	}
	return ""
}

// elementText pulls one value out of a SOAP response without modelling the
// whole envelope, whose namespaces differ between firmwares.
func elementText(body []byte, name string) string {
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != name {
			continue
		}
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return ""
		}
		return value
	}
}

func escapeXML(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
