package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hardhacker/hylo/internal/build"
	"github.com/hardhacker/hylo/internal/config"
)

// Handler holds shared dependencies available to all HTTP handlers.
type Handler struct {
	logger *slog.Logger
	cfg    *config.Config
}

// New creates a Handler with the given dependencies.
func New(logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{logger: logger, cfg: cfg}
}

// HealthCheck handles POST /healthz.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Version handles POST /version.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	info := build.Get()
	// "app" lets clients confirm an arbitrary URL is a Hylo server before
	// asking for a key; /version is unauthenticated, so keep it non-sensitive.
	respondJSON(w, http.StatusOK, map[string]string{
		"app":        "hylo",
		"version":    info.Version,
		"commit":     info.Commit,
		"build_date": info.BuildDate,
	})
}

// respondJSON writes v as JSON with the given status code.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
