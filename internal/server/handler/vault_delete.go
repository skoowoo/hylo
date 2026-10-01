package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type deleteRequest struct {
	Path string `json:"path"`
}

// Delete handles POST /api/vault/delete: permanently delete a note.
// Body: {"path": "/dir/note.md"}
func (gh *VaultHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, `missing required field: "path"`, http.StatusBadRequest)
		return
	}

	p, ok := storage.ParsePath(req.Path)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}

	if err := gh.vault.DeleteNote(p); err != nil {
		writeVaultError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
