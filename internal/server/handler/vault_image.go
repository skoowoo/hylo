package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
)

// writeImageHeaders sets the headers for serving a stored image. The sandbox
// CSP and nosniff keep an SVG opened as a top-level document from running
// scripts on the app origin; <img> embedding is unaffected.
func writeImageHeaders(w http.ResponseWriter, ext, cacheControl string) {
	h := w.Header()
	h.Set("Content-Type", storage.ImageContentType(ext))
	h.Set("Cache-Control", cacheControl)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
}

// UploadImage handles POST /api/vault/upload-image.
// Accepts multipart/form-data with a "file" field containing an image.
// Saves to {vault_root}/_assets/YYYYMM/{unixms}-{rand8}.{ext}.
// Returns {"src": "/_assets/YYYYMM/filename.ext", "name": "filename.ext"}.
func (gh *VaultHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(storage.MaxImageBytes); err != nil {
		http.Error(w, "request parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `missing "file" field: `+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, storage.MaxImageBytes+1))
	if err != nil {
		http.Error(w, "read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(data) > storage.MaxImageBytes {
		http.Error(w, "image too large", http.StatusRequestEntityTooLarge)
		return
	}
	ext := storage.DetectImageExt(data, hdr.Header.Get("Content-Type"), hdr.Filename)
	if ext == "" {
		http.Error(w, "unsupported image type", http.StatusBadRequest)
		return
	}

	img, err := gh.vault.SaveImageBytes(data, ext)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidImageRef) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "write error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"src":  img.Dir + "/" + img.Name,
		"name": img.Name,
	})
}

// ServeAsset handles GET /_assets/{path} — streams image files stored in the
// vault's _assets directory. The .hylo internal directory is always blocked.
func (gh *VaultHandler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	// r.URL.Path is like "/_assets/202501/xxx.png"
	rel := strings.TrimPrefix(r.URL.Path, "/_assets/")
	if rel == "" || strings.Contains(rel, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	root := gh.vault.Root()
	abs := filepath.Join(root, "_assets", filepath.FromSlash(rel))

	// Verify the resolved path stays inside {root}/_assets/.
	assetsRoot := filepath.Join(root, "_assets") + string(os.PathSeparator)
	if !strings.HasPrefix(abs, assetsRoot) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	writeImageHeaders(w, filepath.Ext(abs), "public, max-age=31536000, immutable")
	_, _ = io.Copy(w, f)
}

// ServeImageByName handles GET /api/images/serve?name=<filename>.
// It looks up the image by filename in the metadata DB and streams the file.
func (gh *VaultHandler) ServeImageByName(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}

	imgs, err := gh.vault.GetImagesByName(name)
	if err != nil || len(imgs) == 0 {
		http.NotFound(w, r)
		return
	}

	abs := gh.vault.OsImagePath(imgs[0])
	// Safety check: ensure path stays inside vault root.
	root := gh.vault.Root()
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	writeImageHeaders(w, filepath.Ext(abs), "public, max-age=3600")
	_, _ = io.Copy(w, f)
}

// ServeImageAt handles GET /api/images/at?dir=...&name=...
// Serves a specific image by its vault-relative directory and filename.
func (gh *VaultHandler) ServeImageAt(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	name := r.URL.Query().Get("name")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") ||
		strings.Contains(dir, "..") {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}

	root := gh.vault.Root()
	relDir := strings.TrimPrefix(dir, "/")
	abs := filepath.Join(root, filepath.FromSlash(relDir), name)

	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	writeImageHeaders(w, filepath.Ext(abs), "public, max-age=3600")
	_, _ = io.Copy(w, f)
}

type imageDeleteRequest struct {
	Dir  string `json:"dir"`
	Name string `json:"name"`
}

// DeleteGalleryImage handles POST /api/images/delete — removes the image file
// and its images table row (dir is vault-absolute, e.g. "/_assets/202501").
func (gh *VaultHandler) DeleteGalleryImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req imageDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := gh.vault.DeleteImage(req.Dir, req.Name); err != nil {
		if errors.Is(err, storage.ErrInvalidImageRef) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeVaultError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
