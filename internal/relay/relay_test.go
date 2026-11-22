package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/relayproto"
)

type relayErrBody struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func decodeRelayErr(t *testing.T, rec *httptest.ResponseRecorder) relayErrBody {
	t.Helper()
	var body relayErrBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body, got %q: %v", rec.Body.String(), err)
	}
	if body.RequestID == "" || rec.Header().Get(relayproto.HeaderRequestID) != body.RequestID {
		t.Fatalf("missing or mismatched request_id: body=%q header=%q", body.RequestID, rec.Header().Get(relayproto.HeaderRequestID))
	}
	return body
}

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

	// Missing access key -> 401 relay_unauthorized.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/s/home/api/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}
	if body := decodeRelayErr(t, rec); body.Error != "relay_unauthorized" {
		t.Fatalf("unexpected error code: %+v", body)
	}

	// Wrong key or unknown server -> 404 server_not_found (do not leak server existence).
	for _, path := range []string{"/s/home/api/status", "/s/unknown/api/status"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(relayproto.HeaderKey, "wrong-tok")
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d", path, rec.Code)
		}
		if body := decodeRelayErr(t, rec); body.Error != "server_not_found" {
			t.Fatalf("%s: unexpected error code: %+v", path, body)
		}
	}

	// Valid access key on allowed route while connector offline -> 503 home_offline.
	req := httptest.NewRequest(http.MethodPost, "/s/home/api/status", nil)
	req.Header.Set(relayproto.HeaderKey, "acc-tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when offline, got %d", rec.Code)
	}
	if body := decodeRelayErr(t, rec); body.Error != "home_offline" {
		t.Fatalf("unexpected error code: %+v", body)
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

func TestRelayRoutePolicy(t *testing.T) {
	r, err := New(Config{
		Servers: map[string]ServerCredentials{
			"home": {ConnectorToken: "conn-tok", AccessToken: "acc-tok"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handler()

	allowed := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/s/home/version"},
		{http.MethodPost, "/s/home/api/status"},
		{http.MethodGet, "/s/home/api/live"},
		{http.MethodPost, "/s/home/api/runs"},
		{http.MethodGet, "/s/home/api/runs/run_123/events"},
		{http.MethodGet, "/s/home/api/mates/main"},
		{http.MethodGet, "/s/home/api/chat/uploads/img_1"},
		{http.MethodPost, "/s/home/api/vault/read"},
		{http.MethodGet, "/s/home/api/images/serve"},
	}
	for _, tc := range allowed {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set(relayproto.HeaderKey, "acc-tok")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		// Connector is offline, so passing route policy reaches 503 home_offline.
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: expected 503 (allowed), got %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}

	blocked := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/s/home/api/config"},
		{http.MethodPatch, "/s/home/api/config"},
		{http.MethodPost, "/s/home/api/server/restart"},
		{http.MethodPost, "/s/home/mcp"},
		{http.MethodGet, "/s/home/dev/pulse"},
		{http.MethodGet, "/s/home/home"},
		{http.MethodGet, "/s/home/api/relay/status"},
		{http.MethodPost, "/s/home/api/agents/claude/install"},
		{http.MethodGet, "/s/home/api/status"}, // wrong method
	}
	for _, tc := range blocked {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set(relayproto.HeaderKey, "acc-tok")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403 (blocked), got %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if body := decodeRelayErr(t, rec); body.Error != "route_not_allowed" {
			t.Fatalf("%s %s: unexpected error code: %+v", tc.method, tc.path, body)
		}
	}
}

func TestRelayRejectsAmbiguousPathsAndProtocols(t *testing.T) {
	r, err := New(Config{
		Servers: map[string]ServerCredentials{
			"home": {ConnectorToken: "conn-tok", AccessToken: "acc-tok"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handler()

	// Ambiguous or traversal paths -> 400 invalid_path.
	badURLs := []*url.URL{
		{Path: "/s/home/api/live", RawPath: "/s/home/api%2flive"},
		{Path: "/s/home/api/config", RawPath: "/s/home/api/vault/..%2fconfig"},
		{Path: "/s/home/api/../api/config"},
		{Path: "/s/home//api/live"},
		{Path: "/s/home/api\\live"},
	}
	for _, u := range badURLs {
		req := &http.Request{
			Method: http.MethodGet,
			URL:    u,
			Header: http.Header{relayproto.HeaderKey: []string{"acc-tok"}},
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("url %+v: expected 400, got %d %s", u, rec.Code, rec.Body.String())
		}
		if body := decodeRelayErr(t, rec); body.Error != "invalid_path" {
			t.Fatalf("url %+v: unexpected error code: %+v", u, body)
		}
	}

	// HTTP CONNECT -> 501 unsupported_protocol.
	connReq := httptest.NewRequest(http.MethodConnect, "/s/home/api/live", nil)
	connReq.Header.Set(relayproto.HeaderKey, "acc-tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, connReq)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("CONNECT: expected 501, got %d", rec.Code)
	}
	if body := decodeRelayErr(t, rec); body.Error != "unsupported_protocol" {
		t.Fatalf("CONNECT: unexpected error code: %+v", body)
	}

	// WebSocket Upgrade -> 501 unsupported_protocol.
	upgReq := httptest.NewRequest(http.MethodGet, "/s/home/api/live", nil)
	upgReq.Header.Set(relayproto.HeaderKey, "acc-tok")
	upgReq.Header.Set("Connection", "Upgrade")
	upgReq.Header.Set("Upgrade", "websocket")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, upgReq)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("Upgrade: expected 501, got %d", rec.Code)
	}
	if body := decodeRelayErr(t, rec); body.Error != "unsupported_protocol" {
		t.Fatalf("Upgrade: unexpected error code: %+v", body)
	}
}

func TestRelayPairingAndDeviceRevocation(t *testing.T) {
	stateFile := t.TempDir() + "/relay-state.json"
	r, err := New(Config{
		StateFile: stateFile,
		Servers: map[string]ServerCredentials{
			"home": {ConnectorToken: "conn-tok", AccessToken: "acc-tok"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handler()

	// Create one-time pairing code with Connector credentials.
	createReq := httptest.NewRequest(http.MethodPost, "/relay/v1/pair/create", nil)
	createReq.Header.Set("Authorization", "Bearer conn-tok")
	createReq.Header.Set(relayproto.HeaderServer, "home")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, createReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("pair/create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Code == "" {
		t.Fatalf("invalid create response: %s", rec.Body.String())
	}

	// Redeem the one-time code to receive a per-device token.
	redeemBody := `{"server_id":"home","code":"` + created.Code + `","device_name":"Skoo iPhone"}`
	redeemReq := httptest.NewRequest(http.MethodPost, "/relay/v1/pair/redeem", strings.NewReader(redeemBody))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, redeemReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("pair/redeem status=%d body=%s", rec.Code, rec.Body.String())
	}
	var redeemed struct {
		DeviceID    string `json:"device_id"`
		DeviceToken string `json:"device_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &redeemed); err != nil || redeemed.DeviceID == "" || redeemed.DeviceToken == "" {
		t.Fatalf("invalid redeem response: %s", rec.Body.String())
	}

	// Re-redeeming the same code must fail.
	redeemAgain := httptest.NewRequest(http.MethodPost, "/relay/v1/pair/redeem", strings.NewReader(redeemBody))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, redeemAgain)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on second redeem, got %d", rec.Code)
	}

	// Device token works on /relay/v1/servers/home/status.
	stReq := httptest.NewRequest(http.MethodGet, "/relay/v1/servers/home/status", nil)
	stReq.Header.Set(relayproto.HeaderKey, redeemed.DeviceToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, stReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("device token status=%d body=%s", rec.Code, rec.Body.String())
	}

	// List devices returns metadata without leaking tokens.
	listReq := httptest.NewRequest(http.MethodGet, "/relay/v1/devices", nil)
	listReq.Header.Set("Authorization", "Bearer conn-tok")
	listReq.Header.Set(relayproto.HeaderServer, "home")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, listReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Skoo iPhone") || strings.Contains(rec.Body.String(), redeemed.DeviceToken) {
		t.Fatalf("unexpected devices list: %d %s", rec.Code, rec.Body.String())
	}

	// Revoke the device -> token stops working immediately.
	delReq := httptest.NewRequest(http.MethodDelete, "/relay/v1/devices/"+redeemed.DeviceID, nil)
	delReq.Header.Set("Authorization", "Bearer conn-tok")
	delReq.Header.Set(relayproto.HeaderServer, "home")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, delReq)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", rec.Code, rec.Body.String())
	}

	stReq2 := httptest.NewRequest(http.MethodGet, "/relay/v1/servers/home/status", nil)
	stReq2.Header.Set(relayproto.HeaderKey, redeemed.DeviceToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, stReq2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after revoke, got %d", rec.Code)
	}
}

func TestRelayRepairSameClientReplacesAndSelfRevokes(t *testing.T) {
	r, err := New(Config{
		StateFile: t.TempDir() + "/relay-state.json",
		Servers: map[string]ServerCredentials{
			"home": {ConnectorToken: "conn-tok", AccessToken: "acc-tok"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handler()

	pair := func(clientID string) (string, string) {
		cReq := httptest.NewRequest(http.MethodPost, "/relay/v1/pair/create", nil)
		cReq.Header.Set("Authorization", "Bearer conn-tok")
		cReq.Header.Set(relayproto.HeaderServer, "home")
		cRec := httptest.NewRecorder()
		h.ServeHTTP(cRec, cReq)
		var c struct{ Code string }
		_ = json.Unmarshal(cRec.Body.Bytes(), &c)

		rReq := httptest.NewRequest(http.MethodPost, "/relay/v1/pair/redeem",
			strings.NewReader(`{"server_id":"home","code":"`+c.Code+`","device_name":"iPhone","client_id":"`+clientID+`"}`))
		rRec := httptest.NewRecorder()
		h.ServeHTTP(rRec, rReq)
		if rRec.Code != http.StatusOK {
			t.Fatalf("redeem status=%d body=%s", rRec.Code, rRec.Body.String())
		}
		var d struct {
			DeviceID    string `json:"device_id"`
			DeviceToken string `json:"device_token"`
		}
		_ = json.Unmarshal(rRec.Body.Bytes(), &d)
		return d.DeviceID, d.DeviceToken
	}

	_, tok1 := pair("iphone-uuid-1")
	_, tok2 := pair("iphone-uuid-1")

	// Re-pairing the same client_id invalidates the first token and keeps only one device.
	st1 := httptest.NewRequest(http.MethodGet, "/relay/v1/servers/home/status", nil)
	st1.Header.Set(relayproto.HeaderKey, tok1)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, st1)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected replaced token to return 404, got %d", rec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/relay/v1/devices", nil)
	listReq.Header.Set("Authorization", "Bearer conn-tok")
	listReq.Header.Set(relayproto.HeaderServer, "home")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, listReq)
	var listed struct{ Devices []DeviceInfo }
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	if len(listed.Devices) != 1 {
		t.Fatalf("expected 1 device after re-pairing same client, got %d", len(listed.Devices))
	}

	// Self-revocation removes the device and invalidates tok2.
	selfDel := httptest.NewRequest(http.MethodDelete, "/relay/v1/servers/home/self", nil)
	selfDel.Header.Set(relayproto.HeaderKey, tok2)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, selfDel)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("self revoke status=%d body=%s", rec.Code, rec.Body.String())
	}

	st2 := httptest.NewRequest(http.MethodGet, "/relay/v1/servers/home/status", nil)
	st2.Header.Set(relayproto.HeaderKey, tok2)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, st2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after self revoke, got %d", rec.Code)
	}
}
