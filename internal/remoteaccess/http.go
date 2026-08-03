package remoteaccess

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type requestPeerKey struct{}

// PeerFromRequest returns the authenticated WireGuard peer attached by the
// remote listener. LAN requests deliberately have no peer.
func PeerFromRequest(r *http.Request) (Peer, bool) {
	peer, ok := r.Context().Value(requestPeerKey{}).(Peer)
	return peer, ok
}

// LANOnly protects the historical zero-authentication listener from direct
// public clients. Forwarded headers are deliberately ignored: trusting them
// without a configured proxy boundary would let any caller write its own LAN
// address on a napkin and walk in.
func LANOnly(next http.Handler) http.Handler {
	return lanOnlyWithPrefixes(next, localInterfacePrefixes())
}

func lanOnlyWithPrefixes(next http.Handler, prefixes []netip.Prefix) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			writeRemoteError(w, http.StatusForbidden, "lan_access_required")
			return
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !allowedLANAddress(ip, prefixes) {
			writeRemoteError(w, http.StatusForbidden, "lan_access_required")
			return
		}
		next.ServeHTTP(w, req)
	})
}

func allowedLANAddress(ip netip.Addr, prefixes []netip.Prefix) bool {
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func localInterfacePrefixes() []netip.Prefix {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var prefixes []netip.Prefix
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			raw := address.String()
			slash := strings.LastIndexByte(raw, '/')
			if slash < 0 {
				continue
			}
			host := raw[:slash]
			if zone := strings.LastIndexByte(host, '%'); zone >= 0 {
				host = host[:zone]
			}
			prefix, err := netip.ParsePrefix(host + raw[slash:])
			if err != nil {
				continue
			}
			bits := prefix.Bits()
			if (prefix.Addr().Is4() && bits < 8) || (prefix.Addr().Is6() && bits < 32) {
				continue
			}
			prefixes = append(prefixes, prefix.Masked())
		}
	}
	return prefixes
}

func (r *tunnelRuntime) protect(next http.Handler, httpPort int) http.Handler {
	expectedHost := net.JoinHostPort(TunnelServerAddress, strconv.Itoa(httpPort))
	expectedOrigin := "http://" + expectedHost
	if httpPort == 80 {
		expectedHost = TunnelServerAddress
		expectedOrigin = "http://" + TunnelServerAddress
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Referrer-Policy", "no-referrer")

		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			writeRemoteError(w, http.StatusForbidden, "remote_peer_unknown")
			return
		}
		peer, ok := r.peerByAddress(host)
		if !ok {
			writeRemoteError(w, http.StatusForbidden, "remote_peer_unknown")
			return
		}
		if req.Host != expectedHost {
			// The fixed Host check defeats DNS rebinding. A site resolving its own
			// name to 10.77.0.1 does not become same-origin with Theia by magic.
			writeRemoteError(w, http.StatusMisdirectedRequest, "remote_host_invalid")
			return
		}
		if crossSiteSubresource(req) || (req.Header.Get("Origin") != "" && !sameRemoteOrigin(req, expectedOrigin)) {
			// GET is not automatically harmless: a hostile page can embed a video
			// or image and make an authenticated browser consume Theia's resources.
			// Direct top-level navigation remains possible; subresources do not.
			writeRemoteError(w, http.StatusForbidden, "remote_origin_forbidden")
			return
		}
		if !remoteRouteAllowed(req.Method, req.URL.Path) {
			writeRemoteError(w, http.StatusForbidden, "remote_access_forbidden")
			return
		}
		if requestChangesState(req.Method) && !sameRemoteOrigin(req, expectedOrigin) {
			writeRemoteError(w, http.StatusForbidden, "remote_origin_forbidden")
			return
		}

		ctx := context.WithValue(req.Context(), requestPeerKey{}, peer)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func remoteRouteAllowed(method, path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return method == http.MethodGet || method == http.MethodHead
	}
	if method == http.MethodGet || method == http.MethodHead {
		switch {
		case path == "/api/health":
			return true
		case path == "/api/remote-access/session":
			return true
		// Reading the profile list and its pictures is admitted because a remote
		// device has to know whose history it is writing to: progress carries a
		// profile since M2. Creating, renaming, deleting a profile and replacing
		// a picture are not here, and must not be added -- managing the household
		// stays on the LAN with every other administrative surface.
		case path == "/api/profiles":
			return true
		case strings.HasPrefix(path, "/api/profiles/") && strings.HasSuffix(path, "/avatar"):
			return true
		case strings.HasPrefix(path, "/api/library/"):
			return true
		case strings.HasPrefix(path, "/api/images/"):
			return true
		case strings.HasPrefix(path, "/api/stream/"):
			return true
		default:
			return false
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if (method == http.MethodPut || method == http.MethodDelete) && len(parts) == 5 {
		return parts[0] == "api" && parts[1] == "library" &&
			(parts[2] == "movies" || parts[2] == "episodes") &&
			parts[3] != "" && parts[4] == "progress"
	}
	if method == http.MethodPost && len(parts) == 7 {
		return parts[0] == "api" && parts[1] == "library" &&
			(parts[2] == "movies" || parts[2] == "episodes") &&
			parts[3] != "" && parts[4] == "files" && parts[5] != "" &&
			parts[6] == "inspect"
	}
	return false
}

func crossSiteSubresource(req *http.Request) bool {
	return req.Header.Get("Sec-Fetch-Site") == "cross-site" &&
		req.Header.Get("Sec-Fetch-Mode") != "navigate"
}

func requestChangesState(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func sameRemoteOrigin(req *http.Request, expected string) bool {
	if site := req.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		return false
	}
	origin := req.Header.Get("Origin")
	if origin == "" {
		// WireGuard/native clients do not send Origin. Browsers do on cross-site
		// writes, while the strict Host check above covers DNS rebinding.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme+"://"+parsed.Host == expected
}

func writeRemoteError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
