package connector_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/connector"
	"github.com/hardhacker/hylo/internal/relay"
	"github.com/hardhacker/hylo/internal/relayproto"
)

func TestEndToEndHTTPAndSSEWithJoinKey(t *testing.T) {
	var localConns atomic.Int32
	sseGate := make(chan struct{})

	localMux := http.NewServeMux()
	localMux.HandleFunc("POST /api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"api_key":   r.Header.Get("X-Hylo-API-Key"),
			"relay_key": r.Header.Get(relayproto.HeaderKey),
			"forwarded": r.Header.Get("X-Forwarded-For"),
			"query":     r.URL.Query().Get("q"),
		})
	})
	localMux.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: first\n\n")
		flusher.Flush()
		select {
		case <-sseGate:
			fmt.Fprint(w, "data: second\n\n")
			flusher.Flush()
		case <-r.Context().Done():
		}
	})

	localSrv := httptest.NewUnstartedServer(localMux)
	localSrv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			localConns.Add(1)
		}
	}
	localSrv.Start()
	defer localSrv.Close()

	r, err := relay.New(relay.Config{
		StateFile:    t.TempDir() + "/state.json",
		AutoRegister: true,
		JoinKey:      "my-join-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	relaySrv := httptest.NewServer(r.Handler())
	defer relaySrv.Close()

	// Wrong join key must not come online.
	badCfg := config.RelayConfig{
		Enabled:        true,
		URL:            relaySrv.URL,
		JoinKey:        "wrong-key",
		ServerID:       "srv_bad",
		ConnectorToken: "rlyc_bad",
		AccessToken:    "rlya_bad",
	}
	badConn := connector.New(badCfg, localSrv.Listener.Addr().String(), nil)
	badCtx, badCancel := context.WithCancel(context.Background())
	go func() { _ = badConn.Start(badCtx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && badConn.Status().State != "offline" {
		time.Sleep(20 * time.Millisecond)
	}
	badCancel()
	if badConn.Status().State != "offline" {
		t.Fatalf("expected wrong join key to fail, got state=%q", badConn.Status().State)
	}

	// Correct join key connects and registers.
	goodCfg := config.RelayConfig{
		Enabled:        true,
		URL:            relaySrv.URL,
		JoinKey:        "my-join-secret",
		ServerID:       "srv_home",
		ConnectorToken: "rlyc_home",
		AccessToken:    "rlya_home",
	}
	conn := connector.New(goodCfg, localSrv.Listener.Addr().String(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = conn.Start(ctx)
		close(done)
	}()

	waitFor(t, 3*time.Second, func() bool {
		return conn.Status().State == "online"
	}, "connector did not come online")

	// Idle tunnel connections must not dial local Hylo before a request arrives.
	time.Sleep(100 * time.Millisecond)
	if n := localConns.Load(); n != 0 {
		t.Fatalf("expected 0 local connections while idle, got %d", n)
	}

	// Status endpoint requires access token and reports online.
	statusReq, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/relay/v1/servers/srv_home/status", nil)
	statusReq.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
	statusResp, err := http.DefaultClient.Do(statusReq)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Online bool `json:"online"`
	}
	_ = json.NewDecoder(statusResp.Body).Decode(&st)
	statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK || !st.Online {
		t.Fatalf("status = %d online=%v", statusResp.StatusCode, st.Online)
	}

	// Blocked admin routes never reach local Hylo.
	blockedReq, _ := http.NewRequest(http.MethodPatch, relaySrv.URL+"/s/srv_home/api/config", nil)
	blockedReq.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
	blockedResp, err := http.DefaultClient.Do(blockedReq)
	if err != nil {
		t.Fatal(err)
	}
	blockedResp.Body.Close()
	if blockedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for /api/config, got %d", blockedResp.StatusCode)
	}
	if n := localConns.Load(); n != 0 {
		t.Fatalf("blocked route dialed local Hylo: %d", n)
	}

	// Standard HTTP proxy strips relay/forwarded headers and preserves API key.
	req, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_home/api/status?q=hello", nil)
	req.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
	req.Header.Set("X-Hylo-API-Key", "hylo_secret")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var echoed map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&echoed); err != nil {
		t.Fatal(err)
	}
	if echoed["api_key"] != "hylo_secret" || echoed["relay_key"] != "" || echoed["forwarded"] != "" || echoed["query"] != "hello" {
		t.Fatalf("unexpected echoed payload: %+v", echoed)
	}

	// SSE streams events immediately without waiting for response completion.
	sseReq, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_home/api/live", nil)
	sseReq.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()
	reader := bufio.NewReader(sseResp.Body)
	line1, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line1) != "data: first" {
		t.Fatalf("expected immediate first SSE event, got %q err=%v", line1, err)
	}
	close(sseGate)
	_, _ = reader.ReadString('\n') // blank line
	line2, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line2) != "data: second" {
		t.Fatalf("expected second SSE event, got %q err=%v", line2, err)
	}

	// Disconnecting connector makes Relay report 503 home offline.
	cancel()
	<-done
	waitFor(t, 2*time.Second, func() bool {
		checkReq, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_home/api/status", nil)
		checkReq.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
		checkResp, err := http.DefaultClient.Do(checkReq)
		if err != nil {
			return false
		}
		checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusServiceUnavailable
	}, "relay did not report 503 after connector stopped")
}

func TestPoolReplenishExceedsIdleTargetAndEnforcesMax(t *testing.T) {
	localMux := http.NewServeMux()
	localMux.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: open\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})
	localMux.HandleFunc("POST /api/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	localSrv := httptest.NewServer(localMux)
	defer localSrv.Close()

	r, err := relay.New(relay.Config{
		Servers: map[string]relay.ServerCredentials{
			"srv_pool": {ConnectorToken: "c_pool", AccessToken: "a_pool"},
		},
		IdleTarget:     2,
		MaxConnections: 4,
		AcquireTimeout: 300 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	relaySrv := httptest.NewServer(r.Handler())
	defer relaySrv.Close()

	cfg := config.RelayConfig{
		Enabled:        true,
		URL:            relaySrv.URL,
		ServerID:       "srv_pool",
		ConnectorToken: "c_pool",
		AccessToken:    "a_pool",
	}
	conn := connector.New(cfg, localSrv.Listener.Addr().String(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = conn.Start(ctx) }()

	waitFor(t, 3*time.Second, func() bool {
		return conn.Status().State == "online"
	}, "connector did not come online")

	openSSE := func() *http.Response {
		req, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_pool/api/live", nil)
		req.Header.Set(relayproto.HeaderKey, cfg.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("open SSE: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("open SSE status=%d", resp.StatusCode)
		}
		line, err := bufio.NewReader(resp.Body).ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "data: open" {
			t.Fatalf("unexpected SSE line=%q err=%v", line, err)
		}
		return resp
	}

	// Hold 3 concurrent SSE streams (exceeding IdleTarget=2).
	var streams []*http.Response
	for range 3 {
		streams = append(streams, openSSE())
	}
	defer func() {
		for _, s := range streams {
			s.Body.Close()
		}
	}()

	// A 4th HTTP request must succeed via on-demand replenishment.
	req, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_pool/api/status", nil)
	req.Header.Set(relayproto.HeaderKey, cfg.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 4th request to succeed, got %d", resp.StatusCode)
	}

	// Hold a 4th SSE stream to saturate MaxConnections=4.
	streams = append(streams, openSSE())

	// A 5th request must fail with 429 relay_busy.
	busyReq, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_pool/api/status", nil)
	busyReq.Header.Set(relayproto.HeaderKey, cfg.AccessToken)
	busyResp, err := http.DefaultClient.Do(busyReq)
	if err != nil {
		t.Fatal(err)
	}
	var errBody struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(busyResp.Body).Decode(&errBody)
	busyResp.Body.Close()
	if busyResp.StatusCode != http.StatusTooManyRequests || errBody.Error != "relay_busy" {
		t.Fatalf("expected 429 relay_busy, got %d %+v", busyResp.StatusCode, errBody)
	}

	// Closing one SSE stream frees capacity and triggers replenishment.
	streams[0].Body.Close()
	streams = streams[1:]

	waitFor(t, 2*time.Second, func() bool {
		okReq, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_pool/api/status", nil)
		okReq.Header.Set(relayproto.HeaderKey, cfg.AccessToken)
		okResp, err := http.DefaultClient.Do(okReq)
		if err != nil {
			return false
		}
		okResp.Body.Close()
		return okResp.StatusCode == http.StatusOK
	}, "relay did not recover after SSE stream closed")
}

func TestResponseHeaderTimeoutReturns504(t *testing.T) {
	unblock := make(chan struct{})
	defer close(unblock)

	localMux := http.NewServeMux()
	localMux.HandleFunc("POST /api/status", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	})
	localSrv := httptest.NewServer(localMux)
	defer localSrv.Close()

	r, err := relay.New(relay.Config{
		Servers: map[string]relay.ServerCredentials{
			"srv_slow": {ConnectorToken: "c_slow", AccessToken: "a_slow"},
		},
		ResponseHeaderTimeout: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	relaySrv := httptest.NewServer(r.Handler())
	defer relaySrv.Close()

	cfg := config.RelayConfig{
		Enabled:        true,
		URL:            relaySrv.URL,
		ServerID:       "srv_slow",
		ConnectorToken: "c_slow",
		AccessToken:    "a_slow",
	}
	conn := connector.New(cfg, localSrv.Listener.Addr().String(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = conn.Start(ctx) }()

	waitFor(t, 3*time.Second, func() bool {
		return conn.Status().State == "online"
	}, "connector did not come online")

	req, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/s/srv_slow/api/status", nil)
	req.Header.Set(relayproto.HeaderKey, cfg.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var errBody struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout || errBody.Error != "home_timeout" {
		t.Fatalf("expected 504 home_timeout, got %d %+v", resp.StatusCode, errBody)
	}
}

func TestRevokeDeviceCancelsActiveSSEStream(t *testing.T) {
	localMux := http.NewServeMux()
	localMux.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: connected\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})
	localSrv := httptest.NewServer(localMux)
	defer localSrv.Close()

	r, err := relay.New(relay.Config{
		Servers: map[string]relay.ServerCredentials{
			"srv_dev": {ConnectorToken: "c_dev", AccessToken: "a_dev"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	relaySrv := httptest.NewServer(r.Handler())
	defer relaySrv.Close()

	cfg := config.RelayConfig{
		Enabled:        true,
		URL:            relaySrv.URL,
		ServerID:       "srv_dev",
		ConnectorToken: "c_dev",
		AccessToken:    "a_dev",
	}
	conn := connector.New(cfg, localSrv.Listener.Addr().String(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = conn.Start(ctx) }()

	waitFor(t, 3*time.Second, func() bool {
		return conn.Status().State == "online"
	}, "connector did not come online")

	// Pair a phone.
	pairReq, _ := http.NewRequest(http.MethodPost, relaySrv.URL+"/relay/v1/pair/create", nil)
	pairReq.Header.Set("Authorization", "Bearer c_dev")
	pairReq.Header.Set(relayproto.HeaderServer, "srv_dev")
	pairResp, err := http.DefaultClient.Do(pairReq)
	if err != nil {
		t.Fatal(err)
	}
	var pairData struct{ Code string }
	_ = json.NewDecoder(pairResp.Body).Decode(&pairData)
	pairResp.Body.Close()

	redeemResp, err := http.Post(relaySrv.URL+"/relay/v1/pair/redeem", "application/json",
		strings.NewReader(`{"server_id":"srv_dev","code":"`+pairData.Code+`","device_name":"Test iPhone"}`))
	if err != nil {
		t.Fatal(err)
	}
	var dev struct {
		DeviceID    string `json:"device_id"`
		DeviceToken string `json:"device_token"`
	}
	_ = json.NewDecoder(redeemResp.Body).Decode(&dev)
	redeemResp.Body.Close()
	if dev.DeviceID == "" || dev.DeviceToken == "" {
		t.Fatalf("failed to pair device: %+v", dev)
	}

	// Open an SSE stream with the paired device token.
	sseReq, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_dev/api/live", nil)
	sseReq.Header.Set(relayproto.HeaderKey, dev.DeviceToken)
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()
	reader := bufio.NewReader(sseResp.Body)
	if line, err := reader.ReadString('\n'); err != nil || strings.TrimSpace(line) != "data: connected" {
		t.Fatalf("unexpected first SSE line=%q err=%v", line, err)
	}

	// Revoking the device must terminate the active SSE stream.
	streamDone := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(reader)
		streamDone <- err
	}()

	delReq, _ := http.NewRequest(http.MethodDelete, relaySrv.URL+"/relay/v1/devices/"+dev.DeviceID, nil)
	delReq.Header.Set("Authorization", "Bearer c_dev")
	delReq.Header.Set(relayproto.HeaderServer, "srv_dev")
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke status=%d", delResp.StatusCode)
	}

	select {
	case <-streamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("active SSE stream was not terminated after device revocation")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}
