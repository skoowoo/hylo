package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/config"
)

func TestVersionIdentifiesHyloServer(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), &config.Config{})
	rec := httptest.NewRecorder()
	h.Version(rec, httptest.NewRequest(http.MethodPost, "/version", nil))
	if !strings.Contains(rec.Body.String(), `"app":"hylo"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func testConfigFile(t *testing.T, cfg *config.Config) (*ConfigHTTP, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	m, err := config.ConfigToMap(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteConfigToml(path, m); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewConfigHTTP(logger, cfg, path), path
}

func patchConfig(h *ConfigHTTP, remote string, xff string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	h.Patch(rec, req)
	return rec
}

func TestRemotePatchCannotChangeServerSettings(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 54321, APIKey: "secret"}}
	h, path := testConfigFile(t, cfg)

	rec := patchConfig(h, "10.1.2.3:443", "", `{"patch":{"server":{"host":"0.0.0.0"}}}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("remote patch: %d %s", rec.Code, rec.Body.String())
	}
	rec = patchConfig(h, "127.0.0.1:9", "8.8.8.8", `{"patch":{"server":{"api_key":"stolen"}}}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("proxied patch: %d %s", rec.Code, rec.Body.String())
	}
	rec = patchConfig(h, "127.0.0.1:9", "", `{"patch":{"server":{"host":"0.0.0.0"}}}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("loopback patch: %d %s", rec.Code, rec.Body.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "0.0.0.0") || strings.Contains(string(raw), "stolen") {
		t.Fatalf("server settings were written:\n%s", raw)
	}

	rec = patchConfig(h, "10.1.2.3:443", "", `{"patch":{"vault":{"path":"/tmp/remote-vault"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("vault patch: %d %s", rec.Code, rec.Body.String())
	}
	raw, err = os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), "/tmp/remote-vault") {
		t.Fatalf("vault path not saved: %v\n%s", err, raw)
	}
}

func TestAPIKeyIsNeverRevealed(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 54321, APIKey: "secret"}}
	h, _ := testConfigFile(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config?reveal_secrets=1", nil)
	req.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"api_key":"secret"`) {
		t.Fatal("api key was returned")
	}
}
