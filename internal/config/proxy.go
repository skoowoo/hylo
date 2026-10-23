package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Proxy is the process-wide outbound proxy from server.proxy.
// The zero value connects directly and does not consult the environment.
type Proxy struct {
	u *url.URL
}

// ParseProxy accepts an empty string, or an http or https URL with a host.
func ParseProxy(raw string) (Proxy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Proxy{}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || !proxyScheme(u.Scheme) {
		return Proxy{}, fmt.Errorf("must be an http or https URL with a host")
	}
	// Go matches the scheme case-sensitively.
	u.Scheme = strings.ToLower(u.Scheme)
	return Proxy{u: u}, nil
}

func proxyScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// Func is an http.Transport Proxy. It returns no proxy when p is zero,
// so a client never falls through to ProxyFromEnvironment.
func (p Proxy) Func() func(*http.Request) (*url.URL, error) {
	if p.u == nil {
		return func(*http.Request) (*url.URL, error) { return nil, nil }
	}
	return http.ProxyURL(p.u)
}

// Transport is an HTTP transport that uses p and nothing from the environment.
func (p Proxy) Transport() *http.Transport {
	var tr *http.Transport
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		tr = base.Clone()
	} else {
		tr = &http.Transport{}
	}
	tr.Proxy = p.Func()
	return tr
}

// Addr is the host:port a dial to this proxy uses. Empty when p connects directly.
// Callers that refuse private addresses allow this one, because it is the proxy itself.
func (p Proxy) Addr() string {
	if p.u == nil {
		return ""
	}
	port := p.u.Port()
	if port == "" {
		if strings.ToLower(p.u.Scheme) == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(p.u.Hostname(), port)
}
