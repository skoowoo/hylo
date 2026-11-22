package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/connector"
	"github.com/hardhacker/hylo/internal/qrpng"
	"github.com/hardhacker/hylo/internal/relayproto"
)

type RelayHTTP struct {
	cfg    *config.Config
	source *connector.Connector
	client *http.Client
}

func NewRelayHTTP(cfg *config.Config, source *connector.Connector) *RelayHTTP {
	return &RelayHTTP{
		cfg:    cfg,
		source: source,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *RelayHTTP) Status(w http.ResponseWriter, _ *http.Request) {
	if !h.cfg.Relay.Active() {
		respondJSON(w, http.StatusOK, map[string]any{"enabled": false, "state": "off"})
		return
	}
	if h.source == nil {
		respondJSON(w, http.StatusOK, map[string]any{"enabled": true, "state": "restart_required"})
		return
	}
	s := h.source.Status()
	respondJSON(w, http.StatusOK, map[string]any{"enabled": true, "state": s.State, "error": s.Error, "connected_at": s.ConnectedAt})
}

// PairCreate asks the configured Relay for a short-lived one-time pairing code
// and renders a versioned QR payload for the iOS app to scan.
func (h *RelayHTTP) PairCreate(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Relay.Active() || h.source == nil {
		respondJSON(w, http.StatusConflict, map[string]any{"error": "relay is not active"})
		return
	}
	if strings.TrimSpace(h.cfg.Server.APIKey) == "" {
		respondJSON(w, http.StatusConflict, map[string]any{"error": "server API key is required"})
		return
	}
	relayHTTPBase, err := relayHTTPBaseURL(h.cfg.Relay.URL)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	respBody, status, err := h.doRelayAdmin(r.Context(), http.MethodPost, relayHTTPBase+"/relay/v1/pair/create")
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "cannot reach relay: " + err.Error()})
		return
	}
	if status < 200 || status >= 300 {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": relayErrorSummary(respBody, status)})
		return
	}

	var created struct {
		Code      string `json:"code"`
		ServerID  string `json:"server_id"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil || created.Code == "" {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "invalid response from relay"})
		return
	}

	raw, err := json.Marshal(struct {
		V      int    `json:"v"`
		Relay  string `json:"relay"`
		Server string `json:"server"`
		Code   string `json:"code"`
		APIKey string `json:"api_key"`
	}{
		V:      1,
		Relay:  relayHTTPBase,
		Server: h.cfg.Relay.ServerID,
		Code:   created.Code,
		APIKey: h.cfg.Server.APIKey,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "encode pairing payload"})
		return
	}
	payload := "hylo-pair:v1:" + base64.RawURLEncoding.EncodeToString(raw)
	img, err := qrpng.DataURL(payload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "render QR code"})
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"qrcode_image": img,
		"expires_at":   created.ExpiresAt,
	})
}

func (h *RelayHTTP) DevicesList(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Relay.Active() || h.source == nil {
		respondJSON(w, http.StatusOK, map[string]any{"devices": []any{}})
		return
	}
	relayHTTPBase, err := relayHTTPBaseURL(h.cfg.Relay.URL)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	respBody, status, err := h.doRelayAdmin(r.Context(), http.MethodGet, relayHTTPBase+"/relay/v1/devices")
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "cannot reach relay: " + err.Error()})
		return
	}
	if status < 200 || status >= 300 {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": relayErrorSummary(respBody, status)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (h *RelayHTTP) DeviceRevoke(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Relay.Active() || h.source == nil {
		respondJSON(w, http.StatusConflict, map[string]any{"error": "relay is not active"})
		return
	}
	deviceID := strings.TrimSpace(r.PathValue("id"))
	if deviceID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "missing device id"})
		return
	}
	relayHTTPBase, err := relayHTTPBaseURL(h.cfg.Relay.URL)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	respBody, status, err := h.doRelayAdmin(r.Context(), http.MethodDelete, relayHTTPBase+"/relay/v1/devices/"+url.PathEscape(deviceID))
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "cannot reach relay: " + err.Error()})
		return
	}
	if status < 200 || status >= 300 {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": relayErrorSummary(respBody, status)})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RelayHTTP) doRelayAdmin(ctx context.Context, method, target string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.Relay.ConnectorToken)
	req.Header.Set(relayproto.HeaderServer, h.cfg.Relay.ServerID)
	if jk := strings.TrimSpace(h.cfg.Relay.JoinKey); jk != "" {
		req.Header.Set(relayproto.HeaderJoinKey, jk)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, err
}

func relayHTTPBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid relay URL")
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	case "https", "http":
	default:
		return "", fmt.Errorf("unsupported relay URL scheme")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func relayErrorSummary(body []byte, status int) string {
	var e struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil {
		if e.Message != "" {
			return e.Message
		}
		if e.Error != "" {
			return e.Error
		}
	}
	return fmt.Sprintf("relay returned HTTP %d", status)
}
