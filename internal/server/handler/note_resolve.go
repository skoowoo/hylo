package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

// NewNoteResolve returns an http.Handler for POST /api/notes/resolve.
// It resolves a bare filename to zero or more (dir, name) metadata rows.
func NewNoteResolve(v *storage.Vault) http.Handler {
	return &noteResolveHandler{vault: v}
}

type noteResolveHandler struct {
	vault *storage.Vault
}

type resolveRequest struct {
	Name string `json:"name"`
}

// ServeHTTP handles POST /api/notes/resolve.
// Body: {"name": "note.md"}
// Returns all vault locations where a note with that filename exists.
func (h *noteResolveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "missing required field: name", http.StatusBadRequest)
		return
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.Error(w, "name must be a filename only (no path separators)", http.StatusBadRequest)
		return
	}

	notes, err := h.vault.GetNotesByName(name)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	if notes == nil {
		notes = []storage.Note{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"name":    name,
		"matches": notes,
		"count":   len(notes),
	})
}
