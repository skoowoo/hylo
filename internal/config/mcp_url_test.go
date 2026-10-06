package config

import "testing"

func TestMCPURL(t *testing.T) {
	cases := []struct {
		name string
		s    ServerConfig
		want string
		ok   bool
	}{
		{"default", ServerConfig{Host: "127.0.0.1", Port: 54321}, "http://127.0.0.1:54321/mcp", true},
		{"wildcard bind connects via loopback", ServerConfig{Host: "0.0.0.0", Port: 8080}, "http://127.0.0.1:8080/mcp", true},
		{"tls", ServerConfig{Host: "example.local", Port: 443, CertFile: "c", KeyFile: "k"}, "https://example.local:443/mcp", true},
		{"tcp disabled", ServerConfig{Host: "127.0.0.1", Port: 0}, "", false},
	}
	for _, c := range cases {
		got, ok := c.s.MCPURL()
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}
