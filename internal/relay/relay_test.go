package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/relayproto"
)

func TestAutoRegisterRelayStartsEmpty(t *testing.T) {
	r, err := New(Config{AutoRegister: true, StateFile: t.TempDir() + "/relay-state.json"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("health = %d", rec.Code)
	}
}

func TestRelayAuthAndOfflineStatus(t *testing.T) {
	r, err := New(Config{
		Servers: map[string]ServerCredentials{
			"home": {ConnectorToken: "conn-tok", AccessToken: "acc-tok"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handler()

	// Missing access key -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/home/api/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}

	// Valid access key while connector offline -> 503.
	req := httptest.NewRequest(http.MethodGet, "/s/home/api/status", nil)
	req.Header.Set(relayproto.HeaderKey, "acc-tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when offline, got %d", rec.Code)
	}

	// Status endpoint reports online: false when connector offline.
	stReq := httptest.NewRequest(http.MethodGet, "/relay/v1/servers/home/status", nil)
	stReq.Header.Set(relayproto.HeaderKey, "acc-tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, stReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"online":false`) {
		t.Fatalf("unexpected status response: %d %s", rec.Code, rec.Body.String())
	}
}
