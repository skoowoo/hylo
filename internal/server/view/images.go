package view

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"sort"
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
	LinkedNotesJSON string // JSON array of {name,preview} for data attribute, e.g. [{"name":"note1","preview":"..."}]
	// Caption is the most recently updated linked note's preview text (2-line
	// clamp in CSS); empty → use Name in template (no links, or that note has
	// no excerpt text of its own, e.g. an image-only note).
	Caption string
}

// linkedNoteRef is one entry of imageItem's LinkedNotesJSON — the lightbox's
// "Linked Notes" cards render a note's content excerpt, not just its title,
// so the client needs the preview text alongside the name.
type linkedNoteRef struct {
	Name    string `json:"name"`
	Preview string `json:"preview,omitempty"`
}

// notesPreviewLookup batch-resolves a set of linked-note basenames (without
// ".md") to their cached preview text and recency rank, reusing
// Vault.GetNotesByNames (already ordered by updated_at desc) instead of one
// lookup per image.
func notesPreviewLookup(vault *storage.Vault, basenames []string) (preview map[string]string, rank map[string]int) {
	preview = map[string]string{}
	rank = map[string]int{}
	if len(basenames) == 0 {
		return
	}
	fileNames := make([]string, 0, len(basenames))
	seen := map[string]bool{}
	for _, b := range basenames {
		if b == "" || seen[b] {
			continue
		}
		seen[b] = true
		fileNames = append(fileNames, b+".md")
	}
	notes, err := vault.GetNotesByNames(fileNames)
	if err != nil {
		return
	}
	for i, n := range notes {
		base := strings.TrimSuffix(n.Name, ".md")
		if _, ok := rank[base]; ok {
			continue // duplicate basename across dirs — keep the freshest (first) one
		}
		preview[base] = n.Preview.Text
		rank[base] = i
	}
	return
}

type imagesGridData struct {
	Images []imageItem
	NextNs int64
	// Count is the vault-wide total, for the section header's count badge
	// (homeImagesSectionHTML) — left zero by the pagination-only fragment
	// (imagesGridTemplate), which has no header to put it in.
	Count int
}

func imageItemFrom(img storage.Image, notes []string, preview map[string]string, rank map[string]int) imageItem {
	if notes == nil {
		notes = []string{}
	}

	refs := make([]linkedNoteRef, 0, len(notes))
	bestRank := -1
	caption := ""
	for _, n := range notes {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		refs = append(refs, linkedNoteRef{Name: n, Preview: preview[n]})
		if r, ok := rank[n]; ok && (bestRank == -1 || r < bestRank) {
			bestRank = r
			caption = preview[n]
		}
	}
	sort.SliceStable(refs, func(i, j int) bool {
		ri, oki := rank[refs[i].Name]
		rj, okj := rank[refs[j].Name]
		if !oki {
			ri = len(rank) + 1
		}
		if !okj {
			rj = len(rank) + 1
		}
		return ri < rj
	})
	notesJSON, _ := json.Marshal(refs)

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

// resolveImageLinks batch-fetches, for a set of images, which notes embed
// each one (Vault.NoteNamesForImages, backed by note_assets kind=image) and
// those notes' cached preview text + recency rank (notesPreviewLookup) — the
// two things imageItemFrom needs to show content instead of file metadata.
// A single pair of queries for the whole batch, not one per image.
func resolveImageLinks(vault *storage.Vault, imgs []storage.Image) (linksByImage map[string][]string, preview map[string]string, rank map[string]int) {
	names := make([]string, 0, len(imgs))
	for _, img := range imgs {
		names = append(names, img.Name)
	}
	linksByImage, err := vault.NoteNamesForImages(names)
	if err != nil {
		linksByImage = map[string][]string{}
	}

	var allNotes []string
	for _, notes := range linksByImage {
		allNotes = append(allNotes, notes...)
	}
	preview, rank = notesPreviewLookup(vault, allNotes)
	return
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

	linksByImage, preview, rank := resolveImageLinks(vh.vault, imgs)
	items := make([]imageItem, 0, len(imgs))
	for _, img := range imgs {
		items = append(items, imageItemFrom(img, linksByImage[img.Name], preview, rank))
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
