package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/urlembed"
)

// NewEmbedHandlers returns POST /api/embeds (NDJSON) and POST /api/embeds/progress.
// One resolver so a URL has a single in-flight fetch and one HTTP client.
func NewEmbedHandlers(v *storage.Vault, proxy config.Proxy) (list, progress http.Handler) {
	r := urlembed.NewResolver(v, proxy)
	return &embedHandler{resolver: r}, &embedProgress{resolver: r}
}

type embedHandler struct {
	resolver *urlembed.Resolver
}

type embedRequest struct {
	URLs  []string `json:"urls"`
	Retry bool     `json:"retry"`
}

func (h *embedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req embedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.URLs) > urlembed.MaxURLs {
		http.Error(w, "too many urls", http.StatusBadRequest)
		return
	}
	// One JSON line per URL as it resolves, in no particular order.
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	enc := json.NewEncoder(w)
	h.resolver.ResolveEach(r.Context(), req.URLs, req.Retry, func(res urlembed.Result) {
		if enc.Encode(res) == nil {
			_ = rc.Flush()
		}
	})
}

type embedProgress struct {
	resolver *urlembed.Resolver
}

type progressRequest struct {
	URL        string `json:"url"`
	PositionMS int64  `json:"position_ms"`
}

func (h *embedProgress) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14)
	var req progressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.PositionMS < 0 || req.PositionMS > int64(48*time.Hour/time.Millisecond) {
		http.Error(w, "invalid position", http.StatusBadRequest)
		return
	}
	if err := h.resolver.SetPosition(req.URL, req.PositionMS); err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
