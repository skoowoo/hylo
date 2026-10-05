package view

import (
	"errors"
	"net/http"
	"path"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// ViewHandler serves vault pages as HTML.
type ViewHandler struct {
	vault    *storage.Vault
	searcher search.Searcher
	cfg      *config.Config
}

// NewView creates a ViewHandler for the given Vault.
func NewView(v *storage.Vault, s search.Searcher, cfg *config.Config) *ViewHandler {
	return &ViewHandler{vault: v, searcher: s, cfg: cfg}
}

// writeVaultError maps storage sentinel errors to appropriate HTTP status codes.
func writeVaultError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, storage.ErrInvalidPath),
		errors.Is(err, storage.ErrUnsupportedType),
		errors.Is(err, storage.ErrBinaryContent):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, storage.ErrIsDir), errors.Is(err, storage.ErrIsFile):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
	}
}

// wikilinksExistFunc extracts all wiki link targets from src, batch-queries the vault,
// and returns a function that reports whether a given note filename exists.
// Returns nil when src contains no wiki links (caller treats nil as "all exist").
func (vh *ViewHandler) wikilinksExistFunc(src []byte) func(string) bool {
	names := util.ExtractWikilinkNames(src)
	if len(names) == 0 {
		return nil
	}
	notes, err := vh.vault.GetNotesByNames(names)
	if err != nil {
		return nil
	}
	found := make(map[string]bool, len(notes))
	for _, n := range notes {
		found[n.Name] = true
	}
	return func(name string) bool { return found[name] }
}

// htmxOnly guards fragment-only endpoints. If the request does not carry the
// HX-Request header, the client navigated directly to a fragment URL, so we
// redirect to fallbackURL instead of returning a bare HTML snippet.
// Returns false when a redirect was written; callers must return immediately.
func htmxOnly(w http.ResponseWriter, r *http.Request, fallbackURL string) bool {
	w.Header().Add("Vary", "HX-Request")
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, fallbackURL, http.StatusSeeOther)
		return false
	}
	return true
}

// ensureMarkdownName appends ".md" to a bare name with no extension.
// Names that already carry any extension are returned unchanged so the
// downstream validator can reject unsupported types explicitly.
func ensureMarkdownName(name string) string {
	if path.Ext(name) != "" {
		return name
	}
	return name + ".md"
}
