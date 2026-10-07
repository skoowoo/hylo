package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1": true, "localhost": true, "::1": true, "[::1]": true,
		"": false, "0.0.0.0": false, "192.168.1.5": false, "example.com": false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q)=%v want %v", host, got, want)
		}
	}
}

func TestClientBaseURL(t *testing.T) {
	cases := []struct {
		s    ServerConfig
		want string
	}{
		{ServerConfig{Host: "127.0.0.1", Port: 54321}, "http://127.0.0.1:54321"},
		{ServerConfig{Host: "0.0.0.0", Port: 8080}, "http://127.0.0.1:8080"},
		{ServerConfig{Host: "127.0.0.1", Port: 8080, CertFile: "c", KeyFile: "k"}, "https://127.0.0.1:8080"},
		{ServerConfig{Host: "::1", Port: 8080}, "http://[::1]:8080"},
	}
	for _, c := range cases {
		got, err := c.s.ClientBaseURL()
		if err != nil || got != c.want {
			t.Errorf("%+v => %q, %v; want %q", c.s, got, err, c.want)
		}
	}
	if _, err := (ServerConfig{Port: 0}).ClientBaseURL(); err == nil {
		t.Error("expected error when TCP is disabled")
	}
}

func TestEnvOverridesConfig(t *testing.T) {
	t.Setenv("HYLO_API_KEY", "from-env")
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.APIKey != "from-env" || !cfg.Server.APIKeyFromEnv {
		t.Fatalf("env not applied: %+v", cfg.Server)
	}
}

func TestEnsureAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	cfg := &Config{Server: ServerConfig{Host: "127.0.0.1", Port: 54321}}
	_, gen, err := EnsureAPIKey(cfg, path)
	if err != nil || !gen || !strings.HasPrefix(cfg.Server.APIKey, "hylo_") {
		t.Fatalf("must generate even on loopback: gen=%v key=%q err=%v", gen, cfg.Server.APIKey, err)
	}
	reloaded, _, err := Load(path)
	if err != nil || reloaded.Server.APIKey != cfg.Server.APIKey {
		t.Fatalf("key not persisted: %v %q", err, reloaded.Server.APIKey)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("config perm = %v", st.Mode().Perm())
	}

	again := &Config{Server: ServerConfig{Host: "0.0.0.0", Port: 54321, APIKey: "keep"}}
	if _, gen, _ := EnsureAPIKey(again, path); gen {
		t.Fatal("must not overwrite an existing key")
	}
}
