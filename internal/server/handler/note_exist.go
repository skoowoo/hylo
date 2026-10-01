package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

// maxNoteExistNames caps how many names a single /api/notes/exist request may
// batch, so a pathological wikilink count can't blow up the SQL IN clause.
const maxNoteExistNames = 2000

// NewNoteExist returns an http.Handler for POST /api/notes/exist.
func NewNoteExist(v *storage.Vault) http.Handler {
	return &noteExistHandler{vault: v}
}

type noteExistHandler struct {
	vault *storage.Vault
}

type existRequest struct {
	Names []string `json:"names"`
}

// ServeHTTP handles POST /api/notes/exist.
// Body: {"names": ["a.md", "b.md", ...]}
// Returns the subset of names that exist somewhere in the vault, so callers
// (the wikilink renderer) can treat everything else as broken/deleted.
func (h *noteExistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req existRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Names) > maxNoteExistNames {
		http.Error(w, "too many names in one request", http.StatusBadRequest)
		return
	}
	if len(req.Names) == 0 {
		respondJSON(w, http.StatusOK, map[string]any{"existing": []string{}})
		return
	}

	notes, err := h.vault.GetNotesByNames(req.Names)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	seen := make(map[string]bool, len(notes))
	existing := make([]string, 0, len(notes))
	for _, n := range notes {
		if !seen[n.Name] {
			seen[n.Name] = true
			existing = append(existing, n.Name)
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"existing": existing})
}
