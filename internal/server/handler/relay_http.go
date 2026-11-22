package handler

import (
	"net/http"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/connector"
)

type RelayHTTP struct {
	cfg    *config.Config
	source *connector.Connector
}

func NewRelayHTTP(cfg *config.Config, source *connector.Connector) *RelayHTTP {
	return &RelayHTTP{cfg: cfg, source: source}
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
