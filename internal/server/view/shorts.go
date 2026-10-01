package view

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/hardhacker/vaultr/internal/storage"
	"github.com/hardhacker/vaultr/internal/util"
)

// ─── stream types ─────────────────────────────────────────────────────────────

type shortsStreamGroup struct {
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
	Months           []shortsMonthItem
	ActiveMonthLabel string // "SEP 2026" — the month-picker trigger's own label
}

type shortRenderedEntry struct {
	Time   string
	HTML   template.HTML
	Images []imageItem // attachments, rendered as a grid below HTML — see resolveShortImages
}

// resolveShortImages looks up attachment metadata (for the lightbox's
// data-img-* attributes — same imageItem the /images gallery uses) for each
// ![[name]] embed a short carries. Names that don't resolve (deleted since
// the short was saved) are silently dropped rather than shown broken.
func (vh *ViewHandler) resolveShortImages(names []string) []imageItem {
	if len(names) == 0 {
		return nil
	}
	items := make([]imageItem, 0, len(names))
	for _, name := range names {
		imgs, err := vh.vault.GetImagesByName(name)
		if err != nil || len(imgs) == 0 {
			continue
		}
		items = append(items, imageItemFrom(imgs[0]))
	}
	return items
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func stripShortContent(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "###### Short Note:") {
		if i := strings.Index(content, "\n"); i >= 0 {
			content = strings.TrimSpace(content[i+1:])
		} else {
			content = ""
		}
	}
	return content
}

// loadStreamGroups returns entries grouped by date, newest-first.
// It fetches limit+1 entries to determine whether more exist.
// Returns (groups, cursor, hasMore, err). cursor is the RFC3339 timestamp of
// the oldest entry returned, used as the next load-more `before` value.
func (vh *ViewHandler) loadStreamGroups(before time.Time, limit int) ([]shortsStreamGroup, string, bool, error) {
	entries, err := vh.vault.ListShortEntries(storage.ShortListOptions{
		Before: before,
		Limit:  limit + 1,
	})
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	currentYear := now.Year()
	var groups []shortsStreamGroup
	groupIdx := make(map[string]int)

	for _, e := range entries {
		content := stripShortContent(e.Content)
		text, imageNames := util.ExtractShortImages([]byte(content))
		rendered, renderErr := util.MarkdownToHTMLFragment(text)
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
				DateLabel:   dateLabel,
				IsToday:     date == today,
				IsYesterday: date == yesterday,
			})
		}
		groups[idx].Entries = append(groups[idx].Entries, shortRenderedEntry{
			Time:   e.CreatedAt.Format("15:04"),
			HTML:   template.HTML(rendered), //nolint:gosec // server-rendered markdown
			Images: vh.resolveShortImages(imageNames),
		})
	}

	var cursor string
	if len(entries) > 0 {
		cursor = entries[len(entries)-1].CreatedAt.Format(time.RFC3339)
	}

	return groups, cursor, hasMore, nil
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

	groups, cursor, hasMore, err := vh.loadStreamGroups(before, 30)
	if err != nil {
		http.Error(w, "shorts/stream: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Groups  []shortsStreamGroup
		Cursor  string
		HasMore bool
	}{groups, cursor, hasMore}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := shortsStreamTemplate.Execute(w, data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// ─── templates ────────────────────────────────────────────────────────────────

var shortsStreamTemplate = template.Must(template.New("shorts-stream").Parse(shortsStreamHTML))
