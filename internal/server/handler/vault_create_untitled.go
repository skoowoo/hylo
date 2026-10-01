package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type createUntitledRequest struct {
	// Dir is the vault-absolute directory to create the note in; empty means
	// the vault root ("/").
	Dir     string `json:"dir"`
	Content string `json:"content"`
}

// CreateUntitled handles POST /api/vault/create-untitled: create a new note
// with an auto-generated "Untitled <timestamp>" name and the given initial
// content — the editor calls this once, the first time the user types
// something into a brand-new tab (see content_pane.js's
// __hyloEditorMaterializeTab). There is no draft state before this and no
// filename to pick up front; the note can be renamed any time afterward via
// POST /api/vault/rename.
// Body: {"dir": "/", "content": "..."}
// Response: {"path": "/Untitled 2026-09-26 15-30-00.md"}
func (gh *VaultHandler) CreateUntitled(w http.ResponseWriter, r *http.Request) {
	var req createUntitledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	dir := req.Dir
	if dir == "" {
		dir = "/"
	}
	dirPath, ok := storage.ParsePath(dir)
	if !ok {
		http.Error(w, `"dir" must be an absolute vault path (start with "/")`, http.StatusBadRequest)
		return
	}

	p, err := gh.vault.CreateUntitledNote(dirPath.String(), []byte(req.Content))
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"path": p.String()})
}
