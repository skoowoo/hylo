package view

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
)

// searchResultItem is passed to the search fragment template.
type searchResultItem struct {
	Name        string
	Dir         string
	DirLabel    string // first path segment of Dir for display (empty for vault root)
	UpdatedAt   string
	URL         string
	PreviewPath string
	IsKnowledge bool
	IsIndex     bool
	CanCompile  bool
}

// SearchFragment handles GET /notes/search?q=…&field=… and returns an HTML
// fragment with search results for HTMX to swap into the dropdown list.
// field may be "name", "content", "tag", or "" (search all, default).
func (vh *ViewHandler) SearchFragment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || vh.searcher == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
	for _, res := range results {
		if res.Kind != "knowledge" && res.Kind != "index" {
			if p, ok := storage.ParsePath(storage.JoinPath(res.Dir, res.Name)); ok {
				paths = append(paths, p)
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
		updatedAt := ""
		if !res.UpdatedAt.IsZero() {
			updatedAt = formatRelativeTime(res.UpdatedAt)
		}
		notePath := storage.JoinPath(res.Dir, res.Name)
		q := url.Values{}
		q.Set("name", res.Name)
		isKnowledge := res.Kind == "knowledge"
		isIndex := res.Kind == "index"
		canCompile := !isKnowledge && !isIndex && compileCountByPath[notePath] == 0
		items = append(items, searchResultItem{
			Name:        name,
			Dir:         res.Dir,
			DirLabel:    firstDirSegment(res.Dir),
			UpdatedAt:   updatedAt,
			URL:         "/notes?" + q.Encode(),
			PreviewPath: notePath,
			IsKnowledge: isKnowledge,
			IsIndex:     isIndex,
			CanCompile:  canCompile,
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.ExecuteTemplate(w, "search_results.html", items); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
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

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
