package handler

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/hardhacker/hylo/internal/storage"
)

// NewNoteInfo returns an http.Handler for POST /api/notes/info.
// It batches title/preview/cover lookups by vault path — for callers (e.g.
// the agent chat's "notes touched" cards) that only have a list of paths and
// need enough metadata to render a note-card preview.
func NewNoteInfo(v *storage.Vault) http.Handler {
	return &noteInfoHandler{vault: v}
}

type noteInfoHandler struct {
	vault *storage.Vault
}

type noteInfoRequest struct {
	Paths []string `json:"paths"`
}

type noteInfoRow struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Preview   string `json:"preview,omitempty"`
	Cover     string `json:"cover,omitempty"` // full /api/images/serve URL; empty = no cover
	Dir       string `json:"dir"`
	UpdatedAt int64  `json:"updatedAt"`
}

const noteInfoMaxPaths = 100

// ServeHTTP handles POST /api/notes/info.
// Body: {"paths": ["/journal/april.md", ...]}
// Unknown or malformed paths are silently skipped, matching Vault.GetNotesByPaths.
func (h *noteInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req noteInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Paths) > noteInfoMaxPaths {
		req.Paths = req.Paths[:noteInfoMaxPaths]
	}
	paths := make([]storage.Path, 0, len(req.Paths))
	for _, p := range req.Paths {
		if pp, ok := storage.ParsePath(p); ok {
			paths = append(paths, pp)
		}
	}

	notes, err := h.vault.GetNotesByPaths(paths)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	covers, err := h.vault.NoteAssetFilenames(paths, storage.AssetKindCover)
	if err != nil {
		covers = nil
	}

	rows := make([]noteInfoRow, 0, len(notes))
	for _, n := range notes {
		title := n.Title
		if title == "" {
			title = n.Name
		}
		cover := ""
		if fn := covers[n.PathString()]; fn != "" {
			cover = "/api/images/serve?name=" + url.QueryEscape(fn)
		}
		rows = append(rows, noteInfoRow{
			Path: n.PathString(), Title: title, Preview: n.Preview.Text,
			Cover: cover, Dir: n.Dir, UpdatedAt: n.UpdatedAt.UnixMilli(),
		})
	}
	respondJSON(w, http.StatusOK, map[string]any{"notes": rows})
}
