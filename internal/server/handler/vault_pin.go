package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type pinRequest struct {
	Path   string `json:"path"`
	Pinned bool   `json:"pinned"`
}

// Pin handles POST /api/vault/pin — sets or clears the pinned flag on a note.
func (h *VaultHandler) Pin(w http.ResponseWriter, r *http.Request) {
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	p, ok := storage.ParsePath(req.Path)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}
	if err := h.vault.SetNotePinned(p, req.Pinned); err != nil {
		writeVaultError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

