package config

import (
	"net/http"
	"testing"
)

func TestParseProxy(t *testing.T) {
	p, err := ParseProxy("  ")
	if err != nil || p.Addr() != "" {
		t.Fatalf("empty: %+v %v", p, err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if u, err := p.Func()(req); u != nil || err != nil {
		t.Fatalf("direct = %v %v", u, err)
	}

	p, err = ParseProxy("HTTP://127.0.0.1:7890")
	if err != nil || p.Addr() != "127.0.0.1:7890" {
		t.Fatalf("http: %v %v", p.Addr(), err)
	}
	if u, _ := p.Func()(req); u == nil || u.Scheme != "http" || u.Host != "127.0.0.1:7890" {
		t.Fatalf("func = %v", u)
	}

	for _, raw := range []string{"127.0.0.1:7890", "ftp://127.0.0.1:7890", "socks5://127.0.0.1:1080", "socks5h://127.0.0.1:1080", "http://"} {
		if _, err := ParseProxy(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestValidateProxy(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Proxy = "nope"
	err := Validate(cfg)
	if err == nil || err.Error() == "" {
		t.Fatal("expected server.proxy error")
	}
	cfg.Server.Proxy = "http://127.0.0.1:7890"
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}
