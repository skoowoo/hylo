package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type moveRequest struct {
	Path   string `json:"path"`
	NewDir string `json:"newDir"`
}

// Move handles POST /api/vault/move: move a note to another directory,
// keeping its filename. Body: {"path": "/dir/note.md", "newDir": "/other"}
// Response: {"path": "/other/note.md"}
func (gh *VaultHandler) Move(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, `missing required field: "path"`, http.StatusBadRequest)
		return
	}
	if req.NewDir == "" {
		http.Error(w, `missing required field: "newDir"`, http.StatusBadRequest)
		return
	}

	p, ok := storage.ParsePath(req.Path)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}

	newPath, err := gh.vault.MoveNote(p, req.NewDir)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"path": newPath.String()})
}
