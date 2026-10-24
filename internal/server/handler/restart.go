package handler

import (
	"errors"
	"net/http"

	"github.com/hardhacker/hylo/internal/config"
)

// RestartHTTP serves POST /api/server/restart.
type RestartHTTP struct {
	path    string
	trigger func()
}

func NewRestartHTTP(configPath string, trigger func()) *RestartHTTP {
	return &RestartHTTP{path: configPath, trigger: trigger}
}

// Post checks the config on disk, then asks the process to replace itself.
// The response is written before the trigger so shutdown can finish this request.
func (h *RestartHTTP) Post(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, []string{http.MethodPost})
		return
	}
	cfg, _, err := config.Load(h.path)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := config.Validate(cfg); err != nil {
		var ve config.ValidationError
		if errors.As(err, &ve) {
			respondJSON(w, http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"errors": ve.Errors,
			})
			return
		}
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !cfg.Server.TCPEnabled() {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "server is disabled in the config file"})
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{
		"restarting": true,
		"started_at": processStarted,
	})
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	h.trigger()
}
