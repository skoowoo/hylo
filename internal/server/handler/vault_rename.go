package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

type renameRequest struct {
	Path    string `json:"path"`
	NewName string `json:"newName"`
}

// Rename handles POST /api/vault/rename: rename a note's filename, keeping it
// in the same directory (use /api/vault/move for a directory change).
// Body: {"path": "/dir/old.md", "newName": "new.md"}
// Response: {"path": "/dir/new.md", "renameJobId": 7}
//
// The rename itself has already fully committed by the time this responds.
// renameJobId (0 if it couldn't be enqueued) identifies the async vault-wide
// wikilink/source_notes sweep renamesync runs to fix up other notes that
// referred to the old name — poll it via GET /api/vault/rename-status?id=.
func (gh *VaultHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, `missing required field: "path"`, http.StatusBadRequest)
		return
	}
	newName := strings.TrimSpace(req.NewName)
	if newName == "" {
		http.Error(w, `missing required field: "newName"`, http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(newName, "/\\") {
		http.Error(w, `"newName" must be a filename only (no path separators)`, http.StatusBadRequest)
		return
	}
	newName = ensureMarkdownName(newName)

	p, ok := storage.ParsePath(req.Path)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}

	newPath, jobID, err := gh.vault.RenameNote(p, newName)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"path":        newPath.String(),
		"renameJobId": jobID,
	})
}

// CheckName handles GET /api/vault/check-name?path=<current path>&newName=<candidate>.
// Response: {"available": true}
//
// Purely advisory: it lets the frontend show an inline "name already exists"
// hint while the user is still typing, before they submit. It is NOT the
// authoritative guard — POST /api/vault/rename re-checks name uniqueness
// itself, atomically with the actual rename, so this endpoint being skipped,
// called with stale input, or raced by a concurrent rename can never let a
// colliding rename through.
func (gh *VaultHandler) CheckName(w http.ResponseWriter, r *http.Request) {
	p, ok := storage.ParsePath(r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, `query parameter "path" must be an absolute vault path (start with "/")`, http.StatusBadRequest)
		return
	}
	newName := strings.TrimSpace(r.URL.Query().Get("newName"))
	if newName == "" {
		http.Error(w, `missing required query parameter: "newName"`, http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(newName, "/\\") {
		http.Error(w, `"newName" must be a filename only (no path separators)`, http.StatusBadRequest)
		return
	}
	newName = ensureMarkdownName(newName)

	if newName == p.Base() {
		// Unchanged (or a pure extension-add on the same base) — not a collision.
		respondJSON(w, http.StatusOK, map[string]any{"available": true})
		return
	}

	taken, err := gh.vault.NameTaken(newName)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"available": !taken})
}

// RenameStatus handles GET /api/vault/rename-status?id=<renameJobId>.
// Returns the current progress of the asynchronous sweep enqueued by Rename.
func (gh *VaultHandler) RenameStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if idStr == "" || err != nil {
		http.Error(w, `query parameter "id" must be a rename job id`, http.StatusBadRequest)
		return
	}

	job, err := gh.vault.GetRenameJob(id)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"status":       job.Status,
		"total":        job.Total,
		"done":         job.Done,
		"updatedCount": job.UpdatedCount,
		"error":        job.Error,
	})
}
