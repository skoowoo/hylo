package urlembed

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/config"
)

var errBlocked = errors.New("url not allowed")

// safeClient reaches only public addresses. A direct dial pins the IP it
// checked, so a name cannot resolve public at the check and private at
// connect. Through the configured proxy the proxy resolves the name, so the
// host is checked just before handing off; that leaves the window open in
// exchange for working where sites are only reachable through a proxy.
func safeClient(proxy config.Proxy) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	direct := publicDial(dialer)
	allow := proxy.Addr()
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			pu, err := proxy.Func()(req)
			if pu == nil || err != nil {
				return pu, err
			}
			if err := checkHost(req.Context(), req.URL.Hostname()); err != nil {
				return nil, err
			}
			return pu, nil
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if allow != "" && addr == allow {
				return dialer.DialContext(ctx, network, addr)
			}
			return direct(ctx, network, addr)
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          8,
	}
	return &http.Client{
		Timeout:       15 * time.Second,
		Transport:     transport,
		CheckRedirect: redirectPolicy,
	}
}

func redirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	return validateURL(req.URL)
}

func validateURL(u *url.URL) error {
	if u == nil || u.Hostname() == "" || u.User != nil {
		return errBlocked
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return errBlocked
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	switch host {
	case "localhost", "localhost.localdomain", "metadata.google.internal":
		return errBlocked
	}
	if ip := net.ParseIP(host); ip != nil && !publicIP(ip) {
		return errBlocked
	}
	return nil
}

// checkHost refuses a name that resolves to a non-public address. A failed
// lookup passes: the proxy may resolve what local DNS cannot.
func checkHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if !publicIP(ip) {
			return errBlocked
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if !publicIP(ip.IP) {
			return errBlocked
		}
	}
	return nil
}

func publicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if len(ip) == net.IPv4len {
		for _, n := range reservedV4 {
			if n.Contains(ip) {
				return false
			}
		}
		return true
	}
	// These IPv6 forms carry an IPv4 address and reach it, so that address decides.
	switch {
	case nat64.Contains(ip):
		return publicIP(ip[12:16])
	case sixToFour.Contains(ip):
		return publicIP(ip[2:6])
	case nat64Local.Contains(ip):
		return false
	}
	return true
}

var (
	reservedV4 = cidrs(
		"0.0.0.0/8",       // this network
		"100.64.0.0/10",   // carrier-grade NAT
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // documentation
		"198.51.100.0/24", // documentation
		"203.0.113.0/24",  // documentation
		"240.0.0.0/4",     // reserved, and broadcast
	)
	nat64      = cidrs("64:ff9b::/96")[0]
	nat64Local = cidrs("64:ff9b:1::/48")[0]
	sixToFour  = cidrs("2002::/16")[0]
)

func cidrs(list ...string) []*net.IPNet {
	out := make([]*net.IPNet, len(list))
	for i, s := range list {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			panic(err)
		}
		out[i] = n
	}
	return out
}

func publicDial(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if !publicIP(ip) {
				return nil, errBlocked
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var last error = errBlocked
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
}
