package view

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// searchResultItem is one row of the search overlay's list.
type searchResultItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	DirLabel    string `json:"dirLabel"` // first path segment of the dir ("/" for vault root)
	URL         string `json:"url"`
	Path        string `json:"path"`
	ShortID     string `json:"shortId,omitempty"`
	Stamp       string `json:"stamp,omitempty"`
	Kind        string `json:"kind"`    // "raw", "knowledge", "index" or "short"
	Updated     string `json:"updated"` // list timestamp (formatRelativeTime)
	IsKnowledge bool   `json:"isKnowledge"`
	IsIndex     bool   `json:"isIndex"`
	CanCompile  bool   `json:"canCompile"`
}

// SearchResults handles GET /notes/search?q=…&field=…&kind=… and returns the
// matches as JSON; the overlay renders them keyed by path so rows that survive
// a keystroke keep their DOM. field may be "name", "content", "tag", or ""
// (search all, default).
func (vh *ViewHandler) SearchResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || vh.searcher == nil {
		writeSearchJSON(w, []searchResultItem{})
		return
	}

	field := r.URL.Query().Get("field")
	switch field {
	case "", "name", "content", "tag":
	default:
		field = ""
	}

	kind := r.URL.Query().Get("kind")
	switch kind {
	case "", "raw", "knowledge", "short", "index":
	default:
		kind = ""
	}
	// kind-filter modes search name+content, not a single field
	if kind != "" {
		field = ""
	}

	results, err := vh.searcher.Search(q, search.SearchOptions{
		Limit: 20,
		Type:  field,
		Kind:  kind,
	})
	if err != nil {
		http.Error(w, "search: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// In mixed/name mode, surface filename matches above pure content hits.
	// In tag/content mode bleve scoring is already meaningful; skip the re-sort.
	if field == "" || field == "name" {
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].NameMatch && !results[j].NameMatch
		})
	}

	// Batch-fetch compile_count for all results so we can compute CanCompile accurately.
	// Only raw/short notes need checking; knowledge and index are always non-compilable.
	paths := make([]storage.Path, 0, len(results))
	seenPaths := make(map[storage.Path]struct{}, len(results))
	for _, res := range results {
		if res.Kind != "knowledge" && res.Kind != "index" {
			if p, ok := storage.ParsePath(storage.JoinPath(res.Dir, res.Name)); ok {
				if _, dup := seenPaths[p]; !dup {
					seenPaths[p] = struct{}{}
					paths = append(paths, p)
				}
			}
		}
	}
	compileCountByPath := make(map[string]int, len(paths))
	if len(paths) > 0 {
		if notes, err2 := vh.vault.GetNotesByPaths(paths); err2 == nil {
			for _, n := range notes {
				compileCountByPath[n.PathString()] = n.CompileCount
			}
		}
	}

	items := make([]searchResultItem, 0, len(results))
	for _, res := range results {
		name := strings.TrimSuffix(res.Name, ".md")
		notePath := storage.JoinPath(res.Dir, res.Name)
		params := url.Values{}
		params.Set("name", res.Name)
		isKnowledge := res.Kind == "knowledge"
		isIndex := res.Kind == "index"
		canCompile := !isKnowledge && !isIndex && compileCountByPath[notePath] == 0

		item := searchResultItem{
			Key:         notePath,
			Name:        name,
			DirLabel:    firstDirSegment(res.Dir),
			URL:         "/notes?" + params.Encode(),
			Path:        notePath,
			Kind:        res.Kind,
			Updated:     formatRelativeTimeOrEmpty(res.UpdatedAt),
			IsKnowledge: isKnowledge,
			IsIndex:     isIndex,
			CanCompile:  canCompile,
		}
		if res.ShortID != "" {
			entry, err := vh.vault.GetShort(res.ShortID)
			if err != nil {
				continue
			}
			item.Key = notePath + "#" + res.ShortID
			item.ShortID = res.ShortID
			item.Stamp = storage.ShortStamp(entry.CreatedAt)
			item.DirLabel = ""
			item.Updated = formatShortSearchTime(entry.CreatedAt)
			item.Name = shortSearchSnippet(entry.Content, res.Lines, q)
			if item.Name == "" {
				item.Name = name
			}
		}
		items = append(items, item)
	}

	writeSearchJSON(w, items)
}

func writeSearchJSON(w http.ResponseWriter, items []searchResultItem) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "encode: "+err.Error(), http.StatusInternalServerError)
	}
}

// firstDirSegment returns the top-level directory name for a vault-abs dir path.
// "/" (vault root) is shown as "/".
func firstDirSegment(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "/" {
		return "/"
	}
	if dir == "" {
		return ""
	}
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return ""
	}
	if i := strings.Index(dir, "/"); i >= 0 {
		return dir[:i]
	}
	return dir
}

func formatRelativeTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatRelativeTime(t)
}

// formatRelativeTime is the one list timestamp: the time today, the date
// otherwise, the year only when it isn't this one (chat_reduce.js formatDate matches).
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	t = t.In(now.Location())
	switch {
	case t.YearDay() == now.YearDay() && t.Year() == now.Year():
		return t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2")
	default:
		return t.Format("Jan 2, 2006")
	}
}

func formatShortSearchTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	t = t.In(now.Location())
	switch {
	case t.YearDay() == now.YearDay() && t.Year() == now.Year():
		return t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2 15:04")
	default:
		return t.Format("Jan 2, 2006 15:04")
	}
}

// shortSearchSnippet picks the matched line (or first text line) of a short
// entry, strips markdown/images, and shifts the window with "…" when the match
// sits past the start of the line.
func shortSearchSnippet(content string, hitLines []int, query string) string {
	const maxSnippetRunes = 200
	const leadRunes = 12
	const shiftThreshold = 18

	rawLines := strings.Split(content, "\n")
	cleanLine := func(line string) string {
		t, _ := util.ExtractShortImages([]byte(line))
		if s := strings.TrimSpace(string(t)); strings.HasPrefix(s, "# ") {
			t = []byte(strings.TrimSpace(strings.TrimPrefix(s, "# ")))
		}
		return util.GeneratePreview(t, maxSnippetRunes*4).Text
	}

	picked := ""
	if len(hitLines) > 0 {
		if ln := hitLines[0]; ln >= 1 && ln <= len(rawLines) {
			picked = cleanLine(rawLines[ln-1])
		}
	}
	if picked == "" {
		for _, l := range rawLines {
			if s := cleanLine(l); s != "" {
				picked = s
				break
			}
		}
	}
	if picked == "" {
		return ""
	}

	// Shift window so a match deep in a long line remains visible.
	lowerPicked := strings.ToLower(picked)
	byteIdx := -1
	for _, rawTerm := range strings.Fields(strings.ToLower(query)) {
		term := strings.Trim(rawTerm, `"'#`)
		if term == "" {
			continue
		}
		if idx := strings.Index(lowerPicked, term); idx >= 0 {
			byteIdx = idx
			break
		}
	}
	runes := []rune(picked)
	if byteIdx > 0 {
		runeIdx := utf8.RuneCountInString(picked[:byteIdx])
		if runeIdx > shiftThreshold {
			start := runeIdx - leadRunes
			if start > 0 && start < len(runes) {
				runes = append([]rune("…"), runes[start:]...)
			}
		}
	}
	if len(runes) > maxSnippetRunes {
		return strings.TrimRight(string(runes[:maxSnippetRunes]), " ") + "…"
	}
	return string(runes)
}
