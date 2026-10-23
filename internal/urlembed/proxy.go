package urlembed

import (
	"context"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpproxy"
)

// proxySettings is the user's proxy, read from the login-shell env the agents
// run with; an app launched from the Dock has no proxy vars of its own.
type proxySettings struct {
	fn    func(*url.URL) (*url.URL, error)
	addrs map[string]bool // proxy host:port, the only non-public address a dial may reach
}

func proxyFromEnv(env []string) proxySettings {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(m[k]); v != "" {
				return v
			}
		}
		return ""
	}
	all := get("all_proxy", "ALL_PROXY")
	cfg := httpproxy.Config{
		HTTPProxy:  first(get("http_proxy", "HTTP_PROXY"), all),
		HTTPSProxy: first(get("https_proxy", "HTTPS_PROXY"), all),
		NoProxy:    get("no_proxy", "NO_PROXY"),
	}
	addrs := map[string]bool{}
	for _, raw := range []string{cfg.HTTPProxy, cfg.HTTPSProxy} {
		if addr := proxyAddr(raw); addr != "" {
			addrs[addr] = true
		}
	}
	if len(addrs) == 0 {
		return proxySettings{}
	}
	return proxySettings{fn: cfg.ProxyFunc(), addrs: addrs}
}

// proxyAddr is the host:port the transport dials for a proxy value; a bare
// "host:port" means http, as httpproxy reads it.
func proxyAddr(raw string) string {
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
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
