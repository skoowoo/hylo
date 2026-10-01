package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type statRequest struct {
	// Path is the vault-relative path to the note (must start with "/").
	Path string `json:"path"`
}

// Stat handles POST /api/vault/stat: return metadata for one note from the SQLite index.
//
// Body: {"path": "/notes/april.md"}
//
// Path normalization matches read: a bare filename without extension gets ".md".
func (gh *VaultHandler) Stat(w http.ResponseWriter, r *http.Request) {
	var req statRequest
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
	p, _ = storage.ParsePath(storage.JoinPath(p.Dir(), ensureMarkdownName(p.Base())))

	note, err := gh.vault.StatNote(p)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, note)
}
