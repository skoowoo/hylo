package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hardhacker/hylo/internal/storage"
)

// NewNoteMeta returns an http.Handler for POST /api/notes/meta — the editor's
// "Info" panel: timestamps plus the compile relation of a single note.
func NewNoteMeta(v *storage.Vault) http.Handler {
	return &noteMetaHandler{vault: v}
}

type noteMetaHandler struct {
	vault *storage.Vault
}

type noteMetaRequest struct {
	Path string `json:"path"`
}

type noteMetaLink struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type noteMetaResponse struct {
	Dir          string `json:"dir"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	Size         int64  `json:"size"`
	Kind         string `json:"kind"`
	CompileCount int    `json:"compileCount"`
	// Raw note → knowledge notes compiled from it; knowledge note → its sources.
	Links []noteMetaLink `json:"links"`
}

func (h *noteMetaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req noteMetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	p, ok := storage.ParsePath(req.Path)
	if !ok {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	notes, err := h.vault.GetNotesByPaths([]storage.Path{p})
	if err != nil {
		writeVaultError(w, err)
		return
	}
	if len(notes) == 0 {
		http.Error(w, storage.ErrNotFound.Error(), http.StatusNotFound)
		return
	}
	n := notes[0]

	links, err := h.links(p, n.Kind)
	if err != nil {
		writeVaultError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, noteMetaResponse{
		Dir:          n.Dir,
		CreatedAt:    n.CreatedAt.UnixMilli(),
		UpdatedAt:    n.UpdatedAt.UnixMilli(),
		Size:         n.Size,
		Kind:         string(n.Kind),
		CompileCount: n.CompileCount,
		Links:        links,
	})
}

func (h *noteMetaHandler) links(p storage.Path, kind storage.Kind) ([]noteMetaLink, error) {
	var (
		paths []storage.Path
		err   error
	)
	switch kind {
	case storage.KindIndex:
		return []noteMetaLink{}, nil
	case storage.KindKnowledge:
		paths, err = h.vault.GetKnowledgeDeps(p)
	default:
		paths, err = h.vault.GetSourceKnowledges(p)
	}
	if err != nil {
		return nil, err
	}
	notes, err := h.vault.GetNotesByPaths(paths)
	if err != nil {
		return nil, err
	}
	links := make([]noteMetaLink, 0, len(notes))
	for _, m := range notes {
		title := m.Title
		if title == "" {
			title = m.Name
		}
		links = append(links, noteMetaLink{Path: m.PathString(), Title: title})
	}
	return links, nil
}
