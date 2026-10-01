package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

// readRequest selects the note by either an exact path or a bare filename.
// Exactly one of Path or Name must be set.
type readRequest struct {
	// Path is the vault-relative path to the note (must start with "/").
	// Exact access: the note must exist at this location.
	Path string `json:"path"`
	// Name is the bare filename (e.g. "april.md").
	// Auto-resolve: the vault is searched and the most-recently-updated match is returned.
	Name string `json:"name"`
}

// Read handles POST /api/vault/read.
//
// Exact access:   {"path": "/notes/april.md"}
// Auto-resolve:   {"name": "april.md"}
func (gh *VaultHandler) Read(w http.ResponseWriter, r *http.Request) {
	var req readRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	switch {
	case req.Path != "" && req.Name != "":
		http.Error(w, `specify either "path" or "name", not both`, http.StatusBadRequest)
	case req.Path != "":
		gh.readByPath(w, req.Path)
	case req.Name != "":
		gh.readByName(w, req.Name)
	default:
		http.Error(w, `missing required field: "path" or "name"`, http.StatusBadRequest)
	}
}

// readByPath streams the note at the given absolute vault path.
// No fallback to name-based search is performed.
func (gh *VaultHandler) readByPath(w http.ResponseWriter, pathStr string) {
	p, ok := storage.ParsePath(pathStr)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}
	// Re-parse with ensured extension.
	p, _ = storage.ParsePath(storage.JoinPath(p.Dir(), ensureMarkdownName(p.Base())))

	gh.streamNote(w, p)
}

// readByName finds the most-recently-updated note whose filename equals name
// and streams it. name must not contain path separators.
func (gh *VaultHandler) readByName(w http.ResponseWriter, name string) {
	if strings.ContainsAny(name, "/\\") {
		http.Error(w, `"name" must be a filename only (no path separators)`, http.StatusBadRequest)
		return
	}
	name = ensureMarkdownName(name)

	notes, err := gh.vault.GetNotesByName(name)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	if len(notes) == 0 {
		http.Error(w, storage.ErrNotFound.Error(), http.StatusNotFound)
		return
	}
	// GetNotesByName returns results ordered by updated_at DESC; first is most recent.
	best := notes[0]
	gh.streamNote(w, best.Path())
}

func (gh *VaultHandler) streamNote(w http.ResponseWriter, p storage.Path) {
	rc, err := gh.vault.ReadNoteStream(p)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", contentTypeFor(p.String()))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, rc) //nolint:errcheck
}
