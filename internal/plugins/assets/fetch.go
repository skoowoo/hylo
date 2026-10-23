package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/hardhacker/hylo/internal/config"
)

const (
	fetchTimeout  = 15 * time.Second
	lookupTimeout = 5 * time.Second
)

// remoteExts are the extensions a remote download can be saved under
// (fetchRemote rejects svg and normalises .jpeg to .jpg).
var remoteExts = []string{".jpg", ".png", ".webp", ".gif", ".avif"}

// remoteStem derives the local basename from the source URL, so a repeat
// fetch can find the earlier download by name alone.
func remoteStem(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return "remote-" + hex.EncodeToString(sum[:8])
}

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// blockedIP reports whether ip points at a local or internal network.
func blockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		cgnatPrefix.Contains(ip)
}

// rejectInternalHost fails when host is, or resolves to, an internal address.
// Resolution is separate from the dial (a rebinding race is possible), but
// checking the target keeps a configured proxy working: a dial-time check
// would only ever see the proxy's address.
func rejectInternalHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if blockedIP(ip) {
			return fmt.Errorf("host %s is not allowed", host)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return fmt.Errorf("host %s resolves to a disallowed address", host)
		}
	}
	return nil
}

// guardTransport vets every request, including each redirect hop.
type guardTransport struct {
	base  http.RoundTripper
	check func(host string) error
}

func (t guardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", req.URL.Scheme)
	}
	if err := t.check(req.URL.Hostname()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func newFetchClient(proxy config.Proxy) *http.Client {
	return &http.Client{
		Timeout:   fetchTimeout,
		Transport: guardTransport{base: proxy.Transport(), check: rejectInternalHost},
	}
}
