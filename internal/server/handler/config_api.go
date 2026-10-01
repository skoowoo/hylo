package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/hardhacker/hylo/internal/config"
)

// ConfigHTTP serves GET /api/config, GET /api/config/schema, PATCH /api/config.
type ConfigHTTP struct {
	mu               sync.Mutex
	logger           *slog.Logger
	cfg              *config.Config
	configLoadedPath string // path viper read at startup; may be ""
}

func NewConfigHTTP(logger *slog.Logger, cfg *config.Config, configLoadedPath string) *ConfigHTTP {
	return &ConfigHTTP{
		logger:           logger,
		cfg:              cfg,
		configLoadedPath: configLoadedPath,
	}
}

// Get handles GET /api/config (?reveal_secrets=1 to include secret strings).
func (c *ConfigHTTP) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, []string{http.MethodGet})
		return
	}
	reveal := r.URL.Query().Get("reveal_secrets") == "1" || r.URL.Query().Get("reveal_secrets") == "true"

	writePath, err := config.ResolveConfigWritePath(c.configLoadedPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	values, err := config.ConfigToMap(c.cfg)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	secrets := applySecretMask(values, reveal)

	meta := map[string]any{
		"restart_required_after_save": true,
		"config_write_path":           writePath,
		"config_loaded_path":          c.configLoadedPath,
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"values":  values,
		"secrets": secrets,
		"meta":    meta,
	})
}

func applySecretMask(m map[string]any, reveal bool) map[string]bool {
	sec := map[string]bool{}
	if srv, ok := m["server"].(map[string]any); ok {
		if s, ok := srv["api_key"].(string); ok && s != "" {
			sec["server.api_key"] = true
			if !reveal {
				srv["api_key"] = nil
			}
		}
	}
	plugs, _ := m["plugins"].(map[string]any)
	if plugs != nil {
		if wx, ok := plugs["wechat"].(map[string]any); ok {
			if s, ok := wx["token"].(string); ok && s != "" {
				sec["plugins.wechat.token"] = true
				if !reveal {
					wx["token"] = nil
				}
			}
		}
	}
	return sec
}

// Schema handles GET /api/config/schema .
func (c *ConfigHTTP) Schema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, []string{http.MethodGet})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"fields": staticConfigSchemaFields})
}

type configPatchReq struct {
	Patch map[string]any `json:"patch"`
}

// Patch handles PATCH /api/config with body { "patch": { nested partial } }.
func (c *ConfigHTTP) Patch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		methodNotAllowed(w, []string{http.MethodPatch})
		return
	}

	writePath, err := config.ResolveConfigWritePath(c.configLoadedPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var body configPatchReq
	if err := dec.Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	if body.Patch == nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "missing \"patch\" object"})
		return
	}
	config.NormalizeJSONDecodedMap(body.Patch)

	c.mu.Lock()
	defer c.mu.Unlock()

	baseCfg, err := config.MergedFromOptionalFile(writePath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	baseMap, err := config.ConfigToMap(baseCfg)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	config.MergeJSONIntoTomlRoot(baseMap, body.Patch)

	if err := config.WriteConfigToml(writePath, baseMap); err != nil {
		var ve config.ValidationError
		if errors.As(err, &ve) {
			respondJSON(w, http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"errors": ve.Errors,
			})
			return
		}
		c.logger.Warn("config write failed", "path", writePath, "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"saved":            true,
		"config_file":      writePath,
		"message":          "configuration written; restart the server to apply changes",
		"restart_required": true,
	})
}

func methodNotAllowed(w http.ResponseWriter, allowed []string) {
	w.Header().Set("Allow", joinComma(allowed))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func joinComma(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += ", " + ss[i]
	}
	return out
}
