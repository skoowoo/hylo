package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

// ListDirs handles POST /api/vault/list-dirs — filesystem child directory names for path completion.
// Body: {"path":"/"} optional; omit or "" means vault root.
// Response: {"path":"/journal","dirs":["2026"]}
func (gh *VaultHandler) ListDirs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	dirStr := req.Path
	if dirStr == "" {
		dirStr = "/"
	}
	p, ok := storage.ParsePath(dirStr)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}

	dirs, err := gh.vault.ListChildDirs(p)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"path": p.String(),
		"dirs": dirs,
	})
}
