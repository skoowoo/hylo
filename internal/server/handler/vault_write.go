package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

type writeRequest struct {
	// Path is the vault-relative path for the note (must start with "/").
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append"`
	Prepend bool   `json:"prepend"`
	// Section, when non-empty with Append/Prepend=true, scopes the operation to
	// the matching section heading (case-insensitive).
	Section string `json:"section,omitempty"`
}

// Write handles POST /api/vault/write: create or replace a note.
// Body: {"path": "/dir/note.md", "content": "...", "append": false}
// When append is true the content is appended with smart markdown spacing.
func (gh *VaultHandler) Write(w http.ResponseWriter, r *http.Request) {
	var req writeRequest
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

	data := []byte(req.Content)

	var err error
	switch {
	case req.Append:
		err = gh.vault.AppendNote(p, data, req.Section)
	case req.Prepend:
		err = gh.vault.PrependNote(p, data, req.Section)
	default:
		err = gh.vault.WriteNote(p, data, "")
	}
	if err != nil {
		writeVaultError(w, err)
		return
	}
	note, _ := gh.vault.StatNote(p)
	respondJSON(w, http.StatusOK, note)
}
