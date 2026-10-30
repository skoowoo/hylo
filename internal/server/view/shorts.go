package view

import (
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// ─── stream types ─────────────────────────────────────────────────────────────

type shortsStreamGroup struct {
	Day         string // "2006-01-02"; the composer files a new card under it
	DateLabel   string // "Monday, June 16" or "Monday, June 16, 2025" — group heading
	IsToday     bool
	IsYesterday bool
	Entries     []shortRenderedEntry
}

type shortsMonthItem struct {
	Abbr      string // "JUN"
	Year      string // "2026"
	YM        string // "2026-06"
	IsCurrent bool
}

type shortsStreamPageData struct {
	Groups           []shortsStreamGroup
	Cursor           string // RFC3339 of the oldest entry shown — load-more cursor
	HasMore          bool
	Tag              string   // the #tag the feed is filtered to, "" for all
	Tags             []string // the shorts' most used tags, plus Tag when it isn't among them
	Months           []shortsMonthItem
	ActiveMonthLabel string // "SEP 2026" — the month-picker trigger's own label
}

type shortRenderedEntry struct {
	ID     string
	Hash   string
	Path   string // the daily file
	Stamp  string // the entry's heading time, for jumping to it in the daily file
	Time   string
	Tags   string // space-separated normalized #tags, so live sync can tell a card out of a filtered feed
	HTML   template.HTML
	Quote  bool         // one short line, no images: the grid view sets it large
	Images []shortImage // attachments, rendered below HTML (stream) or as a cover (grid)
	Thumbs int          // grid card's thumb-row columns under the cover
	More   int          // images past the grid card's cover + three thumbs
}

// shortImage carries the image's own aspect ratio so a grid card is laid out
// at its final height before the image loads.
type shortImage struct {
	Name   string
	URL    string
	Aspect string
}

const (
	shortQuoteMaxRunes = 80
	shortGridImages    = 4
	shortsTagRowMax    = 40 // the row shows as many as its width fits
)

// Cover ratios are clamped like a photo feed's: no taller than 3:4, no wider than 3:2.
const (
	shortCoverMinRatio = 3.0 / 4
	shortCoverMaxRatio = 3.0 / 2
)

var shortImageRatios sync.Map // abs path + mtime → float64 width/height

func (vh *ViewHandler) shortImageAspect(img storage.Image) string {
	ratio := 4.0 / 3
	abs := vh.vault.OsImagePath(img)
	key := abs + "@" + img.UpdatedAt.String()
	if v, ok := shortImageRatios.Load(key); ok {
		ratio = v.(float64)
	} else if f, err := os.Open(abs); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil && cfg.Height > 0 {
			ratio = float64(cfg.Width) / float64(cfg.Height)
			shortImageRatios.Store(key, ratio)
		}
		f.Close()
	}
	ratio = min(max(ratio, shortCoverMinRatio), shortCoverMaxRatio)
	return fmt.Sprintf("%.4f", ratio)
}

// resolveShortImages resolves each ![[name]] embed a short carries; names that
// no longer resolve (deleted since) are dropped rather than shown broken.
func (vh *ViewHandler) resolveShortImages(names []string) []shortImage {
	var items []shortImage
	for _, name := range names {
		imgs, err := vh.vault.GetImagesByName(name)
		if err != nil || len(imgs) == 0 {
			continue
		}
		items = append(items, shortImage{imgs[0].Name, imageThumbURL(imgs[0]), vh.shortImageAspect(imgs[0])})
	}
	return items
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// loadStreamGroups returns entries grouped by date, newest-first.
// It fetches limit+1 entries to determine whether more exist.
// Returns (groups, cursor, hasMore, err). cursor is the RFC3339 timestamp of
// the oldest entry returned, used as the next load-more `before` value.
func (vh *ViewHandler) loadStreamGroups(before time.Time, tag string, limit int) ([]shortsStreamGroup, string, bool, error) {
	entries, err := vh.vault.ListShortEntries(storage.ShortListOptions{
		Before: before,
		Tag:    tag,
		Limit:  limit + 1,
	})
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	groups := vh.shortGroups(entries)
	var cursor string
	if len(entries) > 0 {
		cursor = entries[len(entries)-1].CreatedAt.Format(time.RFC3339)
	}

	return groups, cursor, hasMore, nil
}

// shortGroups renders entries (newest first) into day groups.
func (vh *ViewHandler) shortGroups(entries []storage.ShortEntry) []shortsStreamGroup {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	currentYear := now.Year()
	var groups []shortsStreamGroup
	groupIdx := make(map[string]int)

	for _, e := range entries {
		text, imageNames := util.ExtractShortImages([]byte(e.Content))
		rendered, renderErr := util.MarkdownToHTMLFragmentTags(text)
		if renderErr != nil {
			rendered = []byte(template.HTMLEscapeString(string(text)))
		}

		date := e.CreatedAt.Format("2006-01-02")
		idx, ok := groupIdx[date]
		if !ok {
			dateLabel := e.CreatedAt.Format("Monday, January 2")
			if e.CreatedAt.Year() != currentYear {
				dateLabel = e.CreatedAt.Format("Monday, January 2, 2006")
			}
			idx = len(groups)
			groupIdx[date] = idx
			groups = append(groups, shortsStreamGroup{
				Day:         date,
				DateLabel:   dateLabel,
				IsToday:     date == today,
				IsYesterday: date == yesterday,
			})
		}
		images := vh.resolveShortImages(imageNames)
		groups[idx].Entries = append(groups[idx].Entries, shortRenderedEntry{
			ID:     e.ID,
			Hash:   e.Hash,
			Path:   e.Path,
			Stamp:  e.CreatedAt.Format("2006-01-02 15:04:05"),
			Time:   e.CreatedAt.Format("15:04"),
			Tags:   strings.Join(e.Tags, " "),
			HTML:   template.HTML(rendered), //nolint:gosec // server-rendered markdown
			Quote:  len(images) == 0 && !strings.Contains(string(text), "\n") && utf8.RuneCount(text) <= shortQuoteMaxRunes,
			Images: images,
			Thumbs: min(max(len(images)-1, 1), shortGridImages-1),
			More:   max(len(images)-shortGridImages, 0),
		})
	}
	return groups
}

// loadMonthsWithEntries returns the months (within the last 24 calendar
// months) that actually have at least one short note, newest-first, plus the
// current month even when it's empty — these back the "jump to month" select
// in the Shorts header (see homeShortsSectionHTML), so an empty month never
// disappears from it mid-browse. activeYM is the YYYY-MM that should be
// marked selected (typically today's month or the ?from= param value) and is
// likewise always kept even without entries. Skipping empty months keeps the
// list a short, meaningful one instead of 24 mostly-blank options.
func (vh *ViewHandler) loadMonthsWithEntries(activeYM string) []shortsMonthItem {
	notes, _ := vh.vault.ListAllNotes(storage.ListOptions{
		OnlyKinds: []storage.Kind{storage.KindShort},
	})

	monthSet := make(map[string]bool)
	for _, n := range notes {
		date := strings.TrimSuffix(n.Name, ".md")
		if len(date) == 10 {
			monthSet[date[:7]] = true
		}
	}
	monthSet[time.Now().Format("2006-01")] = true
	monthSet[activeYM] = true

	base := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)

	var items []shortsMonthItem
	for i := 0; i < 24; i++ {
		t := base.AddDate(0, -i, 0)
		ym := t.Format("2006-01")
		if !monthSet[ym] {
			continue
		}
		items = append(items, shortsMonthItem{
			Abbr:      strings.ToUpper(t.Format("Jan")),
			Year:      t.Format("2006"),
			YM:        ym,
			IsCurrent: ym == activeYM,
		})
	}
	return items
}

// shortsTagRow is the feed's tag row: the most used tags, with the filtered
// tag kept in it even when it's rarer than those, or on no short at all.
func (vh *ViewHandler) shortsTagRow(selected string) []string {
	counts, _ := vh.vault.ShortTagCounts()
	var tags []string
	found := selected == ""
	for i, c := range counts {
		if i < shortsTagRowMax || c.Tag == selected {
			tags = append(tags, c.Tag)
			found = found || c.Tag == selected
		}
	}
	if !found {
		tags = append(tags, selected)
	}
	return tags
}

// ─── handlers ─────────────────────────────────────────────────────────────────

// ShortsStream handles GET /shorts/stream?before=RFC3339 — HTMX load-more
// fragment, used by home's embedded Shorts section (see home.go).
func (vh *ViewHandler) ShortsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}

	beforeStr := r.URL.Query().Get("before")
	if beforeStr == "" {
		http.Error(w, "before required", http.StatusBadRequest)
		return
	}
	before, err := time.Parse(time.RFC3339, beforeStr)
	if err != nil {
		http.Error(w, "invalid before", http.StatusBadRequest)
		return
	}

	tag := util.NormalizeTag(r.URL.Query().Get("tag"))
	groups, cursor, hasMore, err := vh.loadStreamGroups(before, tag, 30)
	if err != nil {
		http.Error(w, "shorts/stream: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Groups  []shortsStreamGroup
		Cursor  string
		HasMore bool
		Tag     string
	}{groups, cursor, hasMore, tag}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.ExecuteTemplate(w, "shorts_stream.html", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// ShortsEntry handles GET /shorts/entry?id= — one card in its day group, which
// the composer files into the feed itself instead of reloading the section.
func (vh *ViewHandler) ShortsEntry(w http.ResponseWriter, r *http.Request) {
	if !htmxOnly(w, r, "/home") {
		return
	}
	e, err := vh.vault.GetShort(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "shorts/entry: "+err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct{ Groups []shortsStreamGroup }{vh.shortGroups([]storage.ShortEntry{e})}
	if err := views.ExecuteTemplate(w, "shorts-days", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// ShortsTags handles GET /shorts/tags?tag= — the feed's tag row, refetched
// after a short changes so its counts follow the cards.
func (vh *ViewHandler) ShortsTags(w http.ResponseWriter, r *http.Request) {
	if !htmxOnly(w, r, "/home") {
		return
	}
	tag := util.NormalizeTag(r.URL.Query().Get("tag"))
	data := struct {
		Tag  string
		Tags []string
	}{tag, vh.shortsTagRow(tag)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.ExecuteTemplate(w, "shorts-tags", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// ─── templates ────────────────────────────────────────────────────────────────
