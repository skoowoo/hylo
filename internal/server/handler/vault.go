package handler

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

// VaultHandler serves REST operations on vault paths.
// Routes are registered individually in router.go; per-verb logic lives in
// vault_read.go, vault_list.go, vault_stat.go, vault_put.go, and vault_delete.go.
type VaultHandler struct {
	vault *storage.Vault
}

// NewVault creates a VaultHandler backed by the given Vault.
func NewVault(g *storage.Vault) *VaultHandler {
	return &VaultHandler{vault: g}
}

func writeVaultError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, storage.ErrInvalidPath),
		errors.Is(err, storage.ErrUnsupportedType),
		errors.Is(err, storage.ErrBinaryContent):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, storage.ErrIsDir), errors.Is(err, storage.ErrIsFile),
		errors.Is(err, storage.ErrAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, storage.ErrRenameNotAllowed):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ensureMarkdownName appends ".md" to a bare name that carries no file extension.
// A name that already has any extension (including non-markdown ones) is returned unchanged,
// letting the downstream validator reject unsupported types.
func ensureMarkdownName(name string) string {
	if path.Ext(name) != "" {
		return name
	}
	return name + ".md"
}

func contentTypeFor(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
