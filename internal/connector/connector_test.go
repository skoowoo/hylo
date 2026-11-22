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
	localMux.HandleFunc("GET /api/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"api_key":   r.Header.Get("X-Hylo-API-Key"),
			"relay_key": r.Header.Get(relayproto.HeaderKey),
			"forwarded": r.Header.Get("X-Forwarded-For"),
			"query":     r.URL.Query().Get("q"),
		})
	})
	localMux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
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

	// Standard HTTP proxy strips relay/forwarded headers and preserves API key.
	req, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_home/api/echo?q=hello", nil)
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
		t.Fatalf("echo status=%d body=%s", resp.StatusCode, body)
	}
	var echoed map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&echoed); err != nil {
		t.Fatal(err)
	}
	if echoed["api_key"] != "hylo_secret" || echoed["relay_key"] != "" || echoed["forwarded"] != "" || echoed["query"] != "hello" {
		t.Fatalf("unexpected echoed payload: %+v", echoed)
	}

	// SSE streams events immediately without waiting for response completion.
	sseReq, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_home/api/events", nil)
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
		checkReq, _ := http.NewRequest(http.MethodGet, relaySrv.URL+"/s/srv_home/api/echo", nil)
		checkReq.Header.Set(relayproto.HeaderKey, goodCfg.AccessToken)
		checkResp, err := http.DefaultClient.Do(checkReq)
		if err != nil {
			return false
		}
		checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusServiceUnavailable
	}, "relay did not report 503 after connector stopped")
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
