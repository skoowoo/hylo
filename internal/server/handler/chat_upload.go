package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/storage"
)

const (
	maxChatImages     = 5
	maxChatImageBytes = 8 << 20
)

// chatUploadName is the only filename the upload directory serves.
var chatUploadName = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|gif|webp)$`)

var chatImageMIME = map[string]string{
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// ChatUploadPOST handles POST /api/chat/uploads. One image, stored under the
// vault's upload directory, named so a later chat turn can refer to it by id.
func (a *AgentAPI) ChatUploadPOST(w http.ResponseWriter, r *http.Request) {
	// Room for the multipart envelope; saveChatImage enforces the image limit.
	r.Body = http.MaxBytesReader(w, r.Body, maxChatImageBytes+1<<20)
	if err := r.ParseMultipartForm(maxChatImageBytes); err != nil {
		http.Error(w, "request parse error", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `missing "file" field`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	att, err := a.saveChatImage(file, hdr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, att)
}

// ChatUploadGET handles GET /api/chat/uploads/{id}.
func (a *AgentAPI) ChatUploadGET(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.PathValue("id"))
	if !chatUploadName.MatchString(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	abs := filepath.Join(a.uploadRoot(), id)
	f, err := os.Open(abs)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeImageHeaders(w, filepath.Ext(id), "private, max-age=86400")
	http.ServeContent(w, r, id, st.ModTime(), f)
}

func (a *AgentAPI) saveChatImage(file multipart.File, hdr *multipart.FileHeader) (mate.Attachment, error) {
	data, err := io.ReadAll(io.LimitReader(file, maxChatImageBytes+1))
	if err != nil {
		return mate.Attachment{}, fmt.Errorf("read error")
	}
	if len(data) > maxChatImageBytes {
		return mate.Attachment{}, fmt.Errorf("image too large")
	}
	ext := storage.DetectImageExt(data, hdr.Header.Get("Content-Type"), hdr.Filename)
	mime, ok := chatImageMIME[ext]
	if !ok || !storage.CheckImageMagic(data, ext) {
		return mate.Attachment{}, fmt.Errorf("unsupported image type")
	}
	id := uuid.NewString() + ext
	dir := a.uploadRoot()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return mate.Attachment{}, fmt.Errorf("write error")
	}
	abs := filepath.Join(dir, id)
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return mate.Attachment{}, fmt.Errorf("write error")
	}
	return mate.Attachment{ID: id, MIME: mime, Name: cleanAttachmentName(hdr.Filename)}, nil
}

// prepareChatImages checks attachment ids and extra paths, and returns the
// files the agent may read. Ids the client claims were uploaded must exist;
// paths outside the upload directory are dropped.
func (a *AgentAPI) prepareChatImages(atts []mate.Attachment, extra []string) ([]mate.Attachment, []string, error) {
	root := a.uploadRoot()
	var paths []string
	var out []mate.Attachment
	seen := map[string]bool{}
	for _, att := range atts {
		id := filepath.Base(strings.TrimSpace(att.ID))
		if !chatUploadName.MatchString(id) {
			return nil, nil, fmt.Errorf("invalid image")
		}
		abs := filepath.Join(root, id)
		st, err := os.Stat(abs)
		if err != nil || !st.Mode().IsRegular() || st.Size() > maxChatImageBytes {
			return nil, nil, fmt.Errorf("invalid image")
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		paths = append(paths, abs)
		name := id
		if strings.TrimSpace(att.Name) != "" {
			name = cleanAttachmentName(att.Name)
		}
		out = append(out, mate.Attachment{ID: id, MIME: chatImageMIME[strings.ToLower(filepath.Ext(id))], Name: name})
	}
	for _, p := range filterImagePaths(extra, root) {
		if seen[p] {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || st.Size() > maxChatImageBytes {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
		base := filepath.Base(p)
		if chatUploadName.MatchString(base) {
			out = append(out, mate.Attachment{ID: base, MIME: chatImageMIME[strings.ToLower(filepath.Ext(base))], Name: base})
		}
	}
	if len(paths) > maxChatImages {
		return nil, nil, fmt.Errorf("up to %d images per message", maxChatImages)
	}
	return out, paths, nil
}

func cleanAttachmentName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "image"
	}
	if rs := []rune(name); len(rs) > 80 {
		name = string(rs[:80])
	}
	return name
}
