package view

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

const imagesPageSize = 30

type imageItem struct {
	Dir             string
	Name            string
	Ext             string
	Size            string
	UpdatedAt       string
	ThumbURL        string
	CursorNs        int64
	LinkedNotes     []string
	LinkedNotesJSON string // JSON array for data attribute, e.g. ["note1","note2"]
	// Caption is linked note titles joined for the card (2-line clamp in CSS); empty → use Name in template.
	Caption string
}

type imagesGridData struct {
	Images []imageItem
	NextNs int64
	// Count is the vault-wide total, for the section header's count badge
	// (homeImagesSectionHTML) — left zero by the pagination-only fragment
	// (imagesGridTemplate), which has no header to put it in.
	Count int
}

func imageItemFrom(img storage.Image) imageItem {
	notes := img.LinkedNotes
	if notes == nil {
		notes = []string{}
	}
	notesJSON, _ := json.Marshal(notes)
	var captionParts []string
	for _, n := range notes {
		n = strings.TrimSpace(n)
		if n != "" {
			captionParts = append(captionParts, n)
		}
	}
	caption := strings.Join(captionParts, " · ")
	return imageItem{
		Dir:             img.Dir,
		Name:            img.Name,
		Ext:             img.Ext,
		Size:            util.FormatSize(img.Size),
		UpdatedAt:       formatRelativeTime(img.UpdatedAt),
		ThumbURL:        imageThumbURL(img),
		CursorNs:        img.UpdatedAt.UnixNano(),
		LinkedNotes:     notes,
		LinkedNotesJSON: string(notesJSON),
		Caption:         caption,
	}
}

func imageThumbURL(img storage.Image) string {
	if strings.HasPrefix(img.Dir, "/_assets") {
		return img.Dir + "/" + url.PathEscape(img.Name)
	}
	q := url.Values{}
	q.Set("dir", img.Dir)
	q.Set("name", img.Name)
	return "/api/images/at?" + q.Encode()
}

// ImagesGrid handles GET /images/grid?before=NS — HTMX fragment for scroll
// pagination, used by home's embedded Images section (see home.go).
func (vh *ViewHandler) ImagesGrid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}

	var beforeNs int64
	if s := r.URL.Query().Get("before"); s != "" {
		beforeNs, _ = strconv.ParseInt(s, 10, 64)
	}

	imgs, err := vh.vault.ListImages(beforeNs, imagesPageSize+1)
	if err != nil {
		http.Error(w, "images grid: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasMore := len(imgs) > imagesPageSize
	if hasMore {
		imgs = imgs[:imagesPageSize]
	}

	items := make([]imageItem, 0, len(imgs))
	for _, img := range imgs {
		items = append(items, imageItemFrom(img))
	}

	var nextNs int64
	if hasMore && len(items) > 0 {
		nextNs = items[len(items)-1].CursorNs
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := imagesGridTemplate.Execute(w, imagesGridData{Images: items, NextNs: nextNs}); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// ── templates ─────────────────────────────────────────────────────────────────

var imagesGridTemplate = template.Must(template.New("imagesgrid").Parse(imagesGridHTML))
