package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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

// processStarted changes on every restart, since a restart execs a new process.
var processStarted = strconv.FormatInt(time.Now().UnixNano(), 10)

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
		"vault_config_path":           vaultConfigPath(c.cfg),
		"started_at":                  processStarted,
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"values":  values,
		"secrets": secrets,
		"meta":    meta,
	})
}

func vaultConfigPath(cfg *config.Config) string {
	root, err := config.VaultRoot(cfg.Vault.Path)
	if err != nil {
		return ""
	}
	return config.VaultConfigPath(root)
}

func applySecretMask(m map[string]any, reveal bool) map[string]bool {
	sec := map[string]bool{}
	if srv, ok := m["server"].(map[string]any); ok {
		if s, ok := srv["api_key"].(string); ok && s != "" {
			sec["server.api_key"] = true
			// Never returned, even on reveal; `hylo auth show` is the way to read it.
			srv["api_key"] = nil
		}
	}
	for _, key := range secretKeys {
		if maskSecret(m, key, reveal) {
			sec[key] = true
		}
	}
	return sec
}

// secretKeys are the sensitive config fields, hidden unless revealed.
var secretKeys = func() []string {
	var keys []string
	for _, f := range staticConfigSchemaFields {
		if f.Sensitive {
			keys = append(keys, f.Key)
		}
	}
	return keys
}()

// maskSecret reports whether the dotted key holds a value, nulling it unless reveal.
func maskSecret(m map[string]any, key string, reveal bool) bool {
	parts := strings.Split(key, ".")
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			return false
		}
		m = next
	}
	last := parts[len(parts)-1]
	if s, ok := m[last].(string); !ok || s == "" {
		return false
	}
	if !reveal {
		m[last] = nil
	}
	return true
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

	// Listen address, API key, and TLS paths stay out of this API. server.proxy
	// is the one server field the settings UI writes.
	if patchTouchesLockedServer(body.Patch) {
		respondJSON(w, http.StatusForbidden, map[string]any{
			"error": "server settings cannot be changed through the API; edit the config file directly",
		})
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Vault keys go to the vault this server has open, the one the UI showed.
	// Both parts are validated before either file is written.
	vaultPatch, globalPatch := config.SplitVaultPatch(body.Patch)
	var commitVault func() error
	if len(vaultPatch) > 0 {
		root, err := config.VaultRoot(c.cfg.Vault.Path)
		if err == nil {
			commitVault, err = config.PrepareVaultConfig(root, vaultPatch)
		}
		if err != nil {
			c.respondWriteError(w, "vault config", err)
			return
		}
	}
	if len(globalPatch) > 0 {
		if err := c.writeGlobal(writePath, globalPatch); err != nil {
			c.respondWriteError(w, writePath, err)
			return
		}
	}
	if commitVault != nil {
		if err := commitVault(); err != nil {
			c.respondWriteError(w, "vault config", err)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"saved":            true,
		"config_file":      writePath,
		"message":          "configuration written; restart the server to apply changes",
		"restart_required": true,
	})
}

func (c *ConfigHTTP) writeGlobal(writePath string, patch map[string]any) error {
	baseCfg, err := config.MergedFromOptionalFile(writePath)
	if err != nil {
		return err
	}
	baseMap, err := config.ConfigToMap(baseCfg)
	if err != nil {
		return err
	}
	config.MergeJSONIntoTomlRoot(baseMap, patch)
	return config.WriteConfigToml(writePath, baseMap)
}

func (c *ConfigHTTP) respondWriteError(w http.ResponseWriter, what string, err error) {
	var ve config.ValidationError
	if errors.As(err, &ve) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "errors": ve.Errors})
		return
	}
	c.logger.Warn("config write failed", "path", what, "err", err)
	respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
}

func patchTouchesLockedServer(patch map[string]any) bool {
	v, ok := patch["server"]
	if !ok {
		return false
	}
	m, isMap := v.(map[string]any)
	if !isMap {
		return true
	}
	for k := range m {
		if k != "proxy" {
			return true
		}
	}
	return false
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
