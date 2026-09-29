package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hardhacker/vaultr/internal/plugins/search"
	"github.com/hardhacker/vaultr/internal/storage"
)

const homeListPageSize = 20

type homePageData struct {
	Folders        []storage.DirSummary
	IndexNotes     []noteItem
	KnowledgeCount int // total notes under the knowledge dir — the sidebar's "All" count
	SectionHTML    template.HTML
}

// homeSectionData drives the right-hand note list — the content shown for
// whichever sidebar item (Pinned / a folder) is active. Shorts is
// a different "view kind" (the rendered shorts stream, not a note list) and
// is rendered separately by renderHomeShortsSection; see renderHomeSectionHTML.
type homeSectionData struct {
	Title    string
	Count    int
	Items    []noteItem
	NextURL  string // hx-get URL for the load-more sentinel; empty = no more pages
	EmptyMsg string
	// ShowGraphOption/View: Knowledge only. Graph is just a third way to
	// look at the same knowledge notes (alongside list/grid), picked via the
	// seg control — see selectKnowledgeIndex/setListView in home.js.
	ShowGraphOption bool
	View            string // "list" (default) | "grid" | "graph"
}

func (vh *ViewHandler) Home(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	folders, err := vh.vault.ListDirs()
	if err != nil {
		http.Error(w, "home: list dirs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	indexNotes := vh.listIndexItems()

	sectionHTML, err := vh.renderHomeSectionHTML(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("home: render section: %s", err), http.StatusInternalServerError)
		return
	}

	data := homePageData{
		Folders:        folders,
		IndexNotes:     indexNotes,
		KnowledgeCount: vh.knowledgeCount(),
		SectionHTML:    sectionHTML,
	}

	var buf bytes.Buffer
	if err := homePageTemplate.Execute(&buf, data); err != nil {
		http.Error(w, fmt.Sprintf("render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// homeSectionData resolves the query params of a /home, /home/section, or
// /home/section/more request into the note list + head info for the active
// sidebar selection. itemsOnly skips the (slightly more expensive) total-count
// lookups, since /home/section/more only ever re-renders the note rows.
func (vh *ViewHandler) homeSectionData(r *http.Request, itemsOnly bool) (homeSectionData, error) {
	sectionType := r.URL.Query().Get("type")
	var beforeNs int64
	if s := r.URL.Query().Get("before"); s != "" {
		beforeNs, _ = strconv.ParseInt(s, 10, 64)
	}

	var data homeSectionData

	switch sectionType {
	case "folder":
		dirPath := r.URL.Query().Get("path")
		if dirPath == "" || !strings.HasPrefix(dirPath, "/") {
			return data, fmt.Errorf("path must start with /")
		}
		var nextNs int64
		data.Items, nextNs = vh.listDirNoteItems(dirPath, beforeNs, homeListPageSize)
		label := dirPath
		if dirPath != "/" {
			label = strings.TrimPrefix(dirPath, "/")
		}
		data.Title = label
		data.EmptyMsg = "No notes in this folder"
		data.NextURL = homeSectionMoreURL("folder", dirPath, nextNs)
		if !itemsOnly {
			data.Count = vh.dirNoteCount(dirPath)
		}
	case "memory":
		dirPath := "/_memory"
		var nextNs int64
		data.Items, nextNs = vh.listDirNoteItems(dirPath, beforeNs, homeListPageSize)
		data.Title = "Memory"
		data.EmptyMsg = "No memory notes yet"
		data.NextURL = homeSectionMoreURL("memory", "", nextNs)
		if !itemsOnly {
			data.Count = vh.dirNoteCount(dirPath)
		}
	case "knowledge":
		view := r.URL.Query().Get("view")
		if view == "" {
			view = "list"
		}
		data.ShowGraphOption = true
		data.View = view
		data.Title = "Knowledge"
		data.EmptyMsg = "No knowledge notes yet"

		indexParam := strings.TrimSpace(r.URL.Query().Get("index"))
		if indexParam == "" {
			dirPath := "/" + strings.Trim(vh.knowledgeDir(), "/")
			if view != "graph" {
				var nextNs int64
				data.Items, nextNs = vh.listDirNoteItems(dirPath, beforeNs, homeListPageSize)
				data.NextURL = homeSectionMoreURL("knowledge", "", nextNs)
			}
			if !itemsOnly {
				data.Count = vh.knowledgeCount()
			}
		} else {
			// A category: same knowledge-note set the sidebar's Graph-mode
			// filtering already used (GetIndexDeps), just also listable as
			// rows instead of only ever drawn as a graph.
			idxPath, ok := storage.ParsePath(indexParam)
			if !ok {
				return data, fmt.Errorf(`index must be absolute (start with "/")`)
			}
			knowledgePaths, err := vh.vault.GetIndexDeps(idxPath)
			if err != nil {
				return data, err
			}
			if !itemsOnly {
				data.Count = len(knowledgePaths)
			}
			if view != "graph" && len(knowledgePaths) > 0 {
				notes, err := vh.vault.GetNotesByPaths(knowledgePaths)
				if err != nil {
					return data, err
				}
				data.Items = vh.noteItemsFromNotes(notes)
			}
		}
	default: // "pinned"
		pinned, err := vh.vault.ListPinnedNotes()
		if err != nil {
			return data, err
		}
		data.Items = vh.noteItemsFromNotes(pinned)
		data.Title = "Pinned"
		data.EmptyMsg = "No pinned notes"
		data.Count = len(data.Items)
	}
	return data, nil
}

// knowledgeDir returns the configured vault-relative knowledge directory
// (e.g. "_knowledge"), falling back to the documented default when cfg is
// unset (mirrors graph.go's KnowledgeGraphRebuild).
func (vh *ViewHandler) knowledgeDir() string {
	if vh.cfg != nil && vh.cfg.Vault.KnowledgeDir != "" {
		return vh.cfg.Vault.KnowledgeDir
	}
	return "_knowledge"
}

// knowledgeCount returns the total note count under the knowledge dir — the
// sidebar's "All" child count, and the same number homeSectionData's
// unfiltered "knowledge" case uses for its list-head pill.
func (vh *ViewHandler) knowledgeCount() int {
	return vh.dirNoteCount("/" + strings.Trim(vh.knowledgeDir(), "/"))
}

// dirNoteCount looks up dirPath's note count from the full (unfiltered)
// directory list — used for Memory/Knowledge, whose underscore-prefixed
// paths are deliberately excluded from ListDirs() (see Home's .Folders),
// and for a regular folder's sidebar count.
func (vh *ViewHandler) dirNoteCount(dirPath string) int {
	dirs, _ := vh.vault.ListAllDirs()
	for _, d := range dirs {
		if d.Dir == dirPath {
			return d.Count
		}
	}
	return 0
}

func homeSectionMoreURL(typ, path string, nextNs int64) string {
	if nextNs == 0 {
		return ""
	}
	q := url.Values{}
	q.Set("type", typ)
	if path != "" {
		q.Set("path", path)
	}
	q.Set("before", strconv.FormatInt(nextNs, 10))
	return "/home/section/more?" + q.Encode()
}

func renderHomeSectionFull(data homeSectionData) (template.HTML, error) {
	var buf bytes.Buffer
	if err := homeSectionTemplate.ExecuteTemplate(&buf, "full", data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // server-rendered fragment, not user HTML
}

// renderHomeSectionHTML resolves a /home or /home/section request into the
// #home-list-pane contents for whichever sidebar item is active. This is the
// dispatch point between the right pane's different "view kinds": today a
// generic note list (pinned/folder), the rendered Shorts stream, the image
// gallery grid, the tag cloud/picker, the knowledge graph canvas, the inbox
// message list, or the agent chat panel; more kinds can be added here as new
// cases without touching the sidebar's swap mechanism (see home.js
// selectSection/selectFolder).
func (vh *ViewHandler) renderHomeSectionHTML(r *http.Request) (template.HTML, error) {
	switch r.URL.Query().Get("type") {
	case "shorts":
		return vh.renderHomeShortsSection(r)
	case "images":
		return vh.renderHomeImagesSection(r)
	case "tags":
		return vh.renderHomeTagsSection(r)
	case "inbox":
		// Same story as graph: the message list is entirely client-rendered
		// (home.js fetches /api/inbox itself once #home-inbox-list lands in
		// the DOM — see the htmx:afterSwap listener), so this markup never
		// varies with the request.
		return template.HTML(homeInboxSectionHTML), nil //nolint:gosec // static markup, not user HTML
	case "chat":
		// Static shell. home.js attaches the chat kernel once #chat-root
		// lands in the DOM (htmx:afterSwap); the markup itself never varies.
		return template.HTML(homeChatSectionHTML), nil //nolint:gosec // static markup, not user HTML
	}
	data, err := vh.homeSectionData(r, false)
	if err != nil {
		return "", err
	}
	return renderHomeSectionFull(data)
}

// renderHomeShortsSection renders the Shorts view: a header (title + a
// "jump to month" select), a composer, and the date-grouped entry feed
// (shorts.go's loadStreamGroups/loadMonthsWithEntries back both). Pagination
// ("Load earlier") and the month select both stay in place inside
// #home-list-pane via hx-get, rather than navigating anywhere — unlike the
// note-list view kind, they don't go through /home/section/more: the select
// re-requests this same endpoint with &from=, and "Load earlier" hits the
// existing /shorts/stream endpoint directly (its #shorts-stream-groups /
// #shorts-load-more ids match here too, so the fragment it returns drops in
// unchanged).
func (vh *ViewHandler) renderHomeShortsSection(r *http.Request) (template.HTML, error) {
	now := time.Now()
	before := now.Add(time.Second)
	activeYM := now.Format("2006-01")
	activeMonthLabel := strings.ToUpper(now.Format("Jan 2006"))
	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.ParseInLocation("2006-01", from, time.Local); err == nil {
			before = t.AddDate(0, 1, 0) // first moment of next month = end of selected month
			activeYM = from
			activeMonthLabel = strings.ToUpper(t.Format("Jan 2006"))
		}
	}

	groups, cursor, hasMore, err := vh.loadStreamGroups(before, 50)
	if err != nil {
		return "", err
	}

	data := shortsStreamPageData{
		Groups:           groups,
		Cursor:           cursor,
		HasMore:          hasMore,
		Months:           vh.loadMonthsWithEntries(activeYM),
		ActiveMonthLabel: activeMonthLabel,
	}

	var buf bytes.Buffer
	if err := homeShortsSectionTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // server-rendered fragment, not user HTML
}

// renderHomeImagesSection renders the same image gallery grid (toolbar,
// thumbnail grid, select/delete) as the standalone /images page, reusing its
// data loader and item type (images.go). Pagination ("scroll for more") stays
// in place via the existing /images/grid endpoint — its #img-grid target and
// sentinel markup match here too, so no /home-specific pagination is needed.
// The lightbox overlay this grid opens lives once at the page level (see
// homePageHTML) rather than inside this swappable fragment.
func (vh *ViewHandler) renderHomeImagesSection(r *http.Request) (template.HTML, error) {
	imgs, err := vh.vault.ListImages(0, imagesPageSize+1)
	if err != nil {
		return "", err
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

	count, err := vh.vault.CountImages()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := homeImagesSectionTemplate.Execute(&buf, imagesGridData{Images: items, NextNs: nextNs, Count: count}); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // server-rendered fragment, not user HTML
}

// tagWallMaxTags caps how many distinct tags the wall/picker ever fetch —
// generous for "a few hundred tags" without dragging in a vault's entire
// long tail (TagDistribution's own default is 500; a vault with more than
// this many distinct tags still works, it just won't show the extreme tail
// in the wall).
const tagWallMaxTags = 2000

// tagNotesLimit caps how many notes a single tag's filtered list can show.
// Unlike folder/knowledge lists this has no "load more" pagination (mirrors
// Knowledge's index-drilldown, which also fetches its full note set in one
// shot) — a limit this high is just a safety valve, not a real page size.
const tagNotesLimit = 5000

// tagItem is one word in the Tags wall's cloud, or one option in the
// filterable tag picker dropdown shown above a selected tag's note list (see
// homeTagsSectionHTML). JSON tags matter: the wall passes the whole slice to
// the client as JSON (see the "tagsJSON" template func below) for d3-cloud
// to lay out — see __vaultrRenderTagCloud in home.js.
type tagItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// homeTagsData drives the Tags section. Selected == "" renders the wall (all
// tags, weighted by use); Selected != "" renders just that tag's own pill +
// count plus a dropdown for jumping to any other tag, above the filtered
// note list. NextURL always stays empty — see tagNotesLimit — but "rows"
// (homeSectionRowsHTML) references .NextURL unconditionally, so the field
// has to exist here too.
type homeTagsData struct {
	Tags     []tagItem
	Selected string
	Items    []noteItem
	Count    int
	NextURL  string
	EmptyMsg string
}

// renderHomeTagsSection renders the Tags wall (no ?tag=) or, once a tag is
// picked, that tag's filtered note list with a one-line head (current tag +
// count + a single "switch tag" dropdown) instead of listing every tag —
// switching never requires returning to the wall first (see selectTag in
// home.js). Both states share one fetch of the full tag distribution, since
// the picker dropdown reuses the wall's own tag/count data.
func (vh *ViewHandler) renderHomeTagsSection(r *http.Request) (template.HTML, error) {
	data := homeTagsData{EmptyMsg: "No notes with this tag"}

	if vh.searcher != nil {
		counts, err := vh.searcher.TagDistribution(tagWallMaxTags)
		if err != nil {
			return "", err
		}
		data.Tags = make([]tagItem, 0, len(counts))
		for _, c := range counts {
			data.Tags = append(data.Tags, tagItem{Name: c.Tag, Count: int(c.Count)})
		}
	}

	if selected := strings.TrimSpace(r.URL.Query().Get("tag")); selected != "" && vh.searcher != nil {
		data.Selected = selected
		results, err := vh.searcher.Search(selected, search.SearchOptions{Type: "tag", Limit: tagNotesLimit})
		if err != nil {
			return "", err
		}
		paths := make([]storage.Path, 0, len(results))
		for _, res := range results {
			if p, ok := storage.ParsePath(storage.JoinPath(res.Dir, res.Name)); ok {
				paths = append(paths, p)
			}
		}
		if len(paths) > 0 {
			notes, err := vh.vault.GetNotesByPaths(paths)
			if err != nil {
				return "", err
			}
			data.Items = vh.noteItemsFromNotes(notes)
		}
		data.Count = len(data.Items)
	}

	var buf bytes.Buffer
	if err := homeTagsSectionTemplate.ExecuteTemplate(&buf, "tags", data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // server-rendered fragment, not user HTML
}

// HomeSection handles GET /home/section?type=pinned|folder|shorts|images|tags|knowledge|inbox|chat[&path=DIR][&tag=NAME][&index=PATH&view=list|grid|graph]
// Returns the full #home-list-pane contents, used when the sidebar selection changes.
func (vh *ViewHandler) HomeSection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}

	html, err := vh.renderHomeSectionHTML(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// HomeSectionMore handles GET /home/section/more — same params as HomeSection,
// triggered by the load-sentinel at the bottom of the list. Returns only the
// next page of rows + a following sentinel (no head), swapped in via outerHTML.
func (vh *ViewHandler) HomeSectionMore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}

	data, err := vh.homeSectionData(r, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := homeSectionTemplate.ExecuteTemplate(w, "rows", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

var homeTemplateFuncs = template.FuncMap{
	// tagsJSON serializes the wall's tag list for __vaultrRenderTagCloud
	// (home.js) to lay out client-side via d3-cloud. Returned as a plain
	// string, not template.HTML, so html/template still HTML-escapes it into
	// the data-tags attribute (the JSON's own double quotes become &#34;) —
	// the browser unescapes that back to valid JSON before JS reads it.
	"tagsJSON": func(tags []tagItem) string {
		b, err := json.Marshal(tags)
		if err != nil {
			return "[]"
		}
		return string(b)
	},
	"label": func(item noteItem) string {
		if item.Title != "" {
			return item.Title
		}
		return item.Name
	},
	"folderLabel": func(dir string) string {
		if dir == "/" {
			return "/"
		}
		if len(dir) > 1 && dir[0] == '/' {
			return dir[1:]
		}
		return dir
	},
	"encdir": url.QueryEscape,
	"coverURL": func(name string) string {
		if name == "" {
			return ""
		}
		return "/api/images/serve?name=" + url.QueryEscape(name)
	},
	// todoRingDash returns the stroke-dasharray for the todo progress ring
	// (see .home-note-badge--todo-fill): "<filled length> <circumference>",
	// where filled is the arc length representing done/total around a
	// circle of radius todoRingRadius. The gap value only needs to be at
	// least the circle's full circumference — however much filled leaves
	// unused just stays empty, which is exactly the un-done remainder.
	"todoRingDash": func(done, total int) string {
		if total <= 0 {
			return fmt.Sprintf("0 %.2f", todoRingCircumference)
		}
		ratio := float64(done) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
		return fmt.Sprintf("%.2f %.2f", todoRingCircumference*ratio, todoRingCircumference)
	},
}

// todoRingRadius matches the <circle r="..."> in the "rows" template's
// .home-note-badge--todo markup — keep the two in sync if that ever changes.
const todoRingRadius = 8.0

var todoRingCircumference = 2 * math.Pi * todoRingRadius

const homeSectionRowsHTML = `{{define "rows"}}{{range .Items}}
<div class="list-card list-card--clickable home-list-card home-note-row{{if .Cover}} has-cover{{end}}" @click="__vaultrOpenNote($event.currentTarget)"
     draggable="true"
     data-note-path="{{.Path}}" data-note-title="{{label .}}"
     data-note-is-knowledge="{{.IsKnowledge}}" data-note-is-index="{{.IsIndex}}"
     data-note-can-compile="{{.CanCompile}}" data-note-pinned="{{.Pinned}}">
  {{if .Cover}}<div class="home-note-cover" aria-hidden="true"><img src="{{coverURL .Cover}}" alt="" draggable="false" loading="lazy" decoding="async"></div>{{end}}
  <div class="home-note-row-top">
    <div class="home-note-row-titleblock">
      <div class="home-note-row-titlerow">
        <span class="home-note-row-title">{{label .}}</span>
        {{if .TodoTotal}}{{if eq .TodoDone .TodoTotal}}<span class="home-note-badge home-note-badge--done" title="{{.TodoDone}}/{{.TodoTotal}} done"><svg fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></svg></span>{{else}}<span class="home-note-badge home-note-badge--todo" title="{{.TodoDone}}/{{.TodoTotal}}"><svg viewBox="0 0 20 20"><circle class="home-note-badge--todo-track" cx="10" cy="10" r="8"/><circle class="home-note-badge--todo-fill" cx="10" cy="10" r="8" transform="rotate(-90 10 10)" stroke-dasharray="{{todoRingDash .TodoDone .TodoTotal}}"/></svg></span>{{end}}{{end}}
      </div>
      {{if .Preview}}<span class="home-note-row-preview-grid">{{.Preview}}</span>{{end}}
    </div>
    <div class="home-note-row-badges">
      {{if .Pinned}}<span class="home-note-badge home-note-badge--pin" title="Pinned"><svg fill="currentColor" viewBox="0 0 24 24"><path d="M17 3a2 2 0 0 1 2 2v15a1 1 0 0 1-1.496.868l-4.512-2.578a2 2 0 0 0-1.984 0l-4.512 2.578A1 1 0 0 1 5 20V5a2 2 0 0 1 2-2z"/></svg></span>{{end}}
      {{if .IsCompiled}}<span class="home-note-badge home-note-badge--done" title="Compiled"><svg fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></svg></span>{{end}}
    </div>
  </div>
  <div class="home-note-row-meta">
    {{if .Preview}}<span class="home-note-row-preview">{{.Preview}}</span>{{end}}
    <span class="home-note-row-dir">{{.Dir}}</span>
    <span class="home-note-row-time">{{.UpdatedAt}}</span>
  </div>
</div>
{{end}}{{if .NextURL}}
<div class="home-load-sentinel" hx-get="{{.NextURL}}" hx-trigger="intersect once" hx-swap="outerHTML"></div>
{{end}}{{if not .Items}}
<div class="home-list-empty">{{.EmptyMsg}}</div>
{{end}}{{end}}`

const homeSectionFullHTML = `{{define "full"}}<div class="home-list-head">
  <span class="home-list-count" id="home-list-count" title="{{.Title}}">{{.Count}}</span>
  <div class="home-list-head-spacer"></div>
  <div class="seg">
    <button type="button" class="seg-btn" :class="currentListView() === 'list' ? 'active' : ''"
            @click="setListView('list')" title="List view">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <path d="M3 6h18" /><path d="M3 12h18" /><path d="M3 18h18" />
      </svg>
    </button>
    <button type="button" class="seg-btn" :class="currentListView() === 'grid' ? 'active' : ''"
            @click="setListView('grid')" title="Grid view">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <rect width="7" height="7" x="3" y="3" rx="1" /><rect width="7" height="7" x="14" y="3" rx="1" />
        <rect width="7" height="7" x="14" y="14" rx="1" /><rect width="7" height="7" x="3" y="14" rx="1" />
      </svg>
    </button>
    {{if .ShowGraphOption}}
    <button type="button" class="seg-btn" :class="currentListView() === 'graph' ? 'active' : ''"
            @click="setListView('graph')" title="Graph view">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <circle cx="12" cy="4.5" r="2" /><path d="m10.4 6.3-2.8 2.9" /><circle cx="4.5" cy="12" r="2" />
        <path d="M6.5 12h11" /><circle cx="19.5" cy="12" r="2" /><path d="m13.6 15.5 2.8 2.9" /><circle cx="12" cy="19.5" r="2" />
      </svg>
    </button>
    {{end}}
  </div>
</div>
{{if eq .View "graph"}}` + homeGraphSectionHTML + `{{else}}
<div class="home-list-body" id="home-list-body" :class="{'is-grid': currentListView() === 'grid'}">{{template "rows" .}}</div>
{{end}}{{end}}`

var homeSectionTemplate = template.Must(
	template.Must(
		template.New("rows").Funcs(homeTemplateFuncs).Parse(homeSectionRowsHTML),
	).New("full").Funcs(homeTemplateFuncs).Parse(homeSectionFullHTML),
)

// homeTagsSectionHTML is the Tags section: with no tag selected, a
// d3-cloud-packed word cloud of every tag (plain text sized by how often
// it's used — see __vaultrRenderTagCloud, home.js); with one selected, just
// that tag's own pill + a count, plus a single dropdown trigger for jumping
// to another tag (a vault can have hundreds of tags, so listing them all as
// a permanent header row — the first cut of this — doesn't scale; a
// filterable dropdown does). The dropdown reuses the app's shared
// .cselect/.cselect-btn--ghost combo (same component Shorts' month picker
// uses, home.go's homeShortsSectionHTML) with a search input pinned above
// its option list (see .tag-picker-dropdown, home.css) — plain .cselect has
// no such input since none of its other uses need to filter hundreds of
// rows. Switching tags re-requests this same fragment with a different
// &tag=, never routing back through the wall first (see selectTag in
// home.js); "All tags" is folded into the dropdown itself rather than a
// separate always-visible control, to keep the header down to "current tag
// + one control" as asked.
const homeTagsSectionHTML = `{{define "tags"}}{{if .Selected}}<div class="home-list-head">
  <span class="tag-current"><span class="tag-current-name">{{.Selected}}</span></span>
  <span class="home-list-count" id="home-list-count" title="Notes">{{.Count}}</span>
  <div class="home-list-head-spacer"></div>
  <div class="cselect cselect--inline tag-picker" x-data="{tpOpen:false,tpQuery:''}" @click.outside="tpOpen=false">
    <button type="button" class="cselect-btn cselect-btn--ghost" :class="{open:tpOpen}"
            @click="tpOpen=!tpOpen; tpQuery=''; $nextTick(function(){$refs.tpInput && $refs.tpInput.focus()})"
            @keydown.escape="tpOpen=false" aria-label="Switch tag">
      <span class="cselect-btn-text">Switch tag</span>
      <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/></svg>
    </button>
    <div class="cselect-dropdown tag-picker-dropdown" x-show="tpOpen" x-cloak>
      <div class="tag-picker-search">
        <input x-ref="tpInput" type="text" x-model="tpQuery" placeholder="Filter tags…" @keydown.escape="tpOpen=false">
      </div>
      <div class="cselect-divider"></div>
      <button type="button" class="cselect-option tag-picker-alltags" @click="selectSection('tags', '/home/section?type=tags'); tpOpen=false">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
          <rect width="7" height="7" x="3" y="3" rx="1" /><rect width="7" height="7" x="14" y="3" rx="1" />
          <rect width="7" height="7" x="14" y="14" rx="1" /><rect width="7" height="7" x="3" y="14" rx="1" />
        </svg>
        <span>All tags</span>
      </button>
      <div class="cselect-divider"></div>
      <div class="tag-picker-options">
        {{$sel := .Selected}}
        {{range .Tags}}
        <button type="button" class="cselect-option{{if eq .Name $sel}} sel{{end}}"
                x-show="tpQuery === '' || '{{.Name}}'.toLowerCase().includes(tpQuery.toLowerCase())"
                @click="selectTag('{{.Name}}'); tpOpen=false">
          <span class="dot dot--fg cselect-option-dot"></span>
          <span class="tag-picker-option-name">{{.Name}}</span><span class="tag-picker-option-count">{{.Count}}</span>
        </button>
        {{end}}
      </div>
    </div>
  </div>
  <div class="seg">
    <button type="button" class="seg-btn" :class="currentListView() === 'list' ? 'active' : ''"
            @click="setListView('list')" title="List view">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <path d="M3 6h18" /><path d="M3 12h18" /><path d="M3 18h18" />
      </svg>
    </button>
    <button type="button" class="seg-btn" :class="currentListView() === 'grid' ? 'active' : ''"
            @click="setListView('grid')" title="Grid view">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <rect width="7" height="7" x="3" y="3" rx="1" /><rect width="7" height="7" x="14" y="3" rx="1" />
        <rect width="7" height="7" x="14" y="14" rx="1" /><rect width="7" height="7" x="3" y="14" rx="1" />
      </svg>
    </button>
  </div>
</div>
<div class="home-list-body" id="home-list-body" :class="{'is-grid': currentListView() === 'grid'}">{{template "rows" .}}</div>
{{else}}<div class="home-list-head">
  <span class="home-list-count" id="home-list-count" title="Tags">{{len .Tags}}</span>
</div>
<div class="home-list-body" id="home-list-body">{{if .Tags}}
  <div class="tag-cloud" id="tag-cloud" data-tags="{{tagsJSON .Tags}}"></div>
  {{else}}
  <div class="home-list-empty">No tags yet</div>
{{end}}</div>
{{end}}{{end}}`

var homeTagsSectionTemplate = template.Must(homeSectionTemplate.New("tags").Funcs(homeTemplateFuncs).Parse(homeTagsSectionHTML))

// homeShortsSectionHTML lays out Shorts as a single column: the shared
// .home-list-head (title + a "jump to month" picker, in place of the old
// always-visible month rail) above a composer and the date-grouped feed.
// The month picker reuses the app's shared custom-dropdown component
// (.cselect, shared_settings_modal.go) rather than a native <select> — same
// dropdown look/positioning as Settings/Agent Bots, just with the lighter
// "ghost" trigger variant (shared_settings_modal.go's .cselect-btn--ghost)
// since this sits in a header rather than a form field. Each option
// re-requests this same fragment with &from= instead of navigating
// anywhere, same as every other control here.
const homeShortsSectionHTML = `<div class="shorts-view">
  <div class="home-list-head">
    <div class="home-list-head-spacer"></div>
    {{if gt (len .Months) 1}}
    <div class="cselect cselect--inline" x-data="{ csOpen: false }" @click.outside="csOpen = false">
      <button type="button" class="cselect-btn cselect-btn--ghost" :class="{open: csOpen}"
              @click="csOpen = !csOpen" @keydown.escape="csOpen = false" aria-label="Jump to month">
        <span class="cselect-btn-text">{{.ActiveMonthLabel}}</span>
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/></svg>
      </button>
      <div class="cselect-dropdown" x-show="csOpen" x-cloak>
        {{range .Months}}
        <button type="button" class="cselect-option{{if .IsCurrent}} sel{{end}}"
                hx-get="/home/section?type=shorts&from={{.YM}}" hx-target="#home-list-pane" hx-swap="innerHTML"
                @click="csOpen = false">
          <span class="dot dot--fg cselect-option-dot"></span><span>{{.Abbr}} {{.Year}}</span>
        </button>
        {{end}}
      </div>
    </div>
    {{end}}
  </div>

  <div class="shorts-stream-wrap">
    <div class="shorts-stream-inner">
      <div class="shorts-compose-card">
        <textarea id="shorts-compose-input" class="shorts-compose-input" rows="2"
                  placeholder="Write a short…" spellcheck="false" x-model="shortComposeText"
                  :disabled="shortComposeSaving"
                  @keydown="handleShortComposeKeydown($event)"
                  @input="autoResize($event.target)"></textarea>
        <div class="shorts-compose-footer">
          <span class="shorts-compose-hint" x-text="(isMac ? '⌘' : 'Ctrl+') + '↵ to save'"></span>
          <button type="button" class="shorts-compose-send"
                  :disabled="!shortComposeText.trim() || shortComposeSaving"
                  @click="saveShortCompose()">Save</button>
        </div>
      </div>
      <div id="shorts-stream-groups">
        {{range .Groups}}
        {{$group := .}}
        <div class="shorts-day">
          {{range .Entries}}
          <div class="list-card shorts-entry">
            <div class="shorts-entry-meta">
              <span class="shorts-entry-date">{{if $group.IsToday}}Today{{else if $group.IsYesterday}}Yesterday{{else}}{{$group.DateLabel}}{{end}}</span>
              <span class="shorts-entry-time">{{.Time}}</span>
            </div>
            <div class="prose shorts-entry-prose">{{.HTML}}</div>
          </div>
          {{end}}
        </div>
        {{else}}
        <div class="shorts-empty-state">
          <div class="shorts-empty-icon">
            <svg fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24"><path d="M13 21h8"/><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/></svg>
          </div>
          <p class="shorts-empty-label">No shorts yet</p>
          <button type="button" class="shorts-empty-btn"
                  onclick="var el=document.getElementById('shorts-compose-input'); if (el) el.focus();">
            <svg fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" viewBox="0 0 24 24"><path d="M5 12h14"/><path d="M12 5v14"/></svg>Write a short
          </button>
        </div>
        {{end}}
      </div>

      <div id="shorts-load-more">
        {{if .HasMore}}
        <div class="shorts-load-more-wrap">
          <button class="shorts-loadmore-btn"
                  hx-get="/shorts/stream?before={{.Cursor}}"
                  hx-target="#shorts-stream-groups"
                  hx-swap="beforeend">
            Load earlier
          </button>
        </div>
        {{end}}
      </div>
    </div>
  </div>
</div>`

var homeShortsSectionTemplate = template.Must(template.New("home-shorts-section").Parse(homeShortsSectionHTML))

// homeImagesSectionHTML mirrors images.html's toolbar + scroll + grid
// verbatim — same markup, ids, and hx-get pagination target as the
// standalone /images page, just without its own .img-main wrapper (the
// #home-list-pane flex column already plays that role).
const homeImagesSectionHTML = `<div class="home-list-head">
  <span class="home-list-count" title="Images">{{.Count}}</span>
  <div class="home-list-head-spacer"></div>
  <button type="button" class="btn-outline btn--xs" x-show="!selectMode" @click="enterSelectMode()">
    <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path stroke-linecap="round" stroke-linejoin="round" d="m9 12 2 2 4-4"/></svg>
    Select
  </button>
  <div class="toolbar-select-group" x-show="selectMode" style="display:none">
    <div class="bulk-count"><strong x-text="selectedCount"></strong> selected</div>
    <button type="button" class="btn-solid btn-solid--danger btn--xs"
            :disabled="!selectedCount"
            @click="deleteSelected()"
            x-text="selectedCount ? 'Delete (' + selectedCount + ')' : 'Delete'">
    </button>
    <button type="button" class="btn-outline btn--xs" @click="exitSelectMode()">Cancel</button>
  </div>
</div>
<div class="img-scroll">
  <div class="img-grid" id="img-grid">
    {{- if .Images}}
    {{- range .Images}}
    <div class="list-card list-card--clickable img-card" onclick="openImageLightbox(this)"
         data-img-src="{{.ThumbURL}}" data-img-name="{{.Name}}" data-img-dir="{{.Dir}}"
         data-img-size="{{.Size}}" data-img-time="{{.UpdatedAt}}" data-img-ext="{{.Ext}}"
         data-img-notes="{{.LinkedNotesJSON}}">
      <div class="img-thumb-wrap">
        <img class="img-thumb" src="{{.ThumbURL}}" alt="{{.Name}}" loading="lazy" decoding="async">
        <div class="img-select-check"><svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg></div>
        {{- if .LinkedNotes}}
        <div class="img-link-badge">
          <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z"/><path stroke-linecap="round" stroke-linejoin="round" d="M14 2v5a1 1 0 0 0 1 1h5"/></svg>
          {{len .LinkedNotes}}
        </div>
        {{- end}}
      </div>
      <div class="img-info">
        <div class="img-caption">{{if .Caption}}{{.Caption}}{{else}}{{.Name}}{{end}}</div>
        <div class="img-time">{{.UpdatedAt}}</div>
      </div>
    </div>
    {{- end}}
    {{- if .NextNs}}
    <div class="img-sentinel"
         hx-get="/images/grid?before={{.NextNs}}"
         hx-trigger="intersect once"
         hx-swap="beforebegin"
         hx-target="this"></div>
    {{- end}}
    {{- else}}
    <div class="img-empty empty-state">
      <div class="empty-state-icon">
        <svg fill="none" stroke="currentColor" stroke-width="1.25" viewBox="0 0 48 48">
          <rect x="5" y="8" width="38" height="32" rx="3"/>
          <circle cx="16" cy="18" r="3.5"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 33l10-10 8 8 7-9 13 13"/>
        </svg>
      </div>
      <div class="empty-state-title">No images yet</div>
      <div class="empty-state-desc">Images you add to your notes will show up here.</div>
    </div>
    {{- end}}
  </div>
</div>`

var homeImagesSectionTemplate = template.Must(template.New("home-images-section").Parse(homeImagesSectionHTML))

// homeImagesLightboxHTML mirrors images.go's lightbox overlay verbatim. It
// lives once at the page level (spliced into homePageHTML below), not inside
// the swappable #home-list-pane, since it's a full-viewport modal driven by
// homeCtrl's lightbox/selectMode state regardless of which sidebar section
// is currently showing.
const homeImagesLightboxHTML = `
  <div class="lb-overlay"
       x-show="lightbox"
       x-transition:enter="lb-enter"
       x-transition:enter-start="lb-enter-start"
       x-transition:enter-end="lb-enter-end"
       x-transition:leave="lb-leave"
       x-transition:leave-start="lb-leave-start"
       x-transition:leave-end="lb-leave-end"
       @click.self="closeLightbox()"
       style="display:none">
    <div class="lb-panel" @click.stop>
      <div class="lb-viewer">
        <img class="lb-viewer-img"
             :src="lightbox ? lightbox.src : ''"
             :alt="lightbox ? lightbox.name : ''"
             draggable="false">
      </div>
      <aside class="lb-sidebar">
        <div class="lb-sidebar-head">
          <span class="lb-sidebar-title" x-text="lightbox ? lightbox.name : ''"></span>
          <button type="button" class="lb-delete-btn" title="Delete image"
                  x-show="lightbox"
                  @click="deleteLightboxImage()">
            <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18"/><path stroke-linecap="round" stroke-linejoin="round" d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path stroke-linecap="round" stroke-linejoin="round" d="M10 11v6"/><path stroke-linecap="round" stroke-linejoin="round" d="M14 11v6"/></svg>
          </button>
          <button type="button" class="icon-btn-ghost icon-btn-ghost--sm" @click="closeLightbox()" title="Close (Esc)">
            <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" d="M18 6 6 18"/><path stroke-linecap="round" d="m6 6 12 12"/></svg>
          </button>
        </div>
        <div class="lb-sidebar-body">
          <div class="lb-section-label">Linked Notes</div>
          <template x-if="lightbox && lightbox.notes && lightbox.notes.length > 0">
            <div>
              <template x-for="note in lightbox.notes" :key="note">
                <div class="lb-note-card" @click="openLinkedNote(note)">
                  <svg class="lb-note-card-icon" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z"/><path stroke-linecap="round" stroke-linejoin="round" d="M14 2v5a1 1 0 0 0 1 1h5"/></svg>
                  <span class="lb-note-card-name" x-text="note"></span>
                  <svg class="lb-note-card-arrow" fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 18l6-6-6-6"/>
                  </svg>
                </div>
              </template>
            </div>
          </template>
          <template x-if="!lightbox || !lightbox.notes || lightbox.notes.length === 0">
            <div class="lb-no-notes">No linked notes</div>
          </template>
          <div class="lb-divider"></div>
          <div class="lb-section-label">Info</div>
          <div class="lb-field">
            <div class="lb-field-label">Filename</div>
            <div class="lb-field-value mono" x-text="lightbox ? lightbox.name : ''"></div>
          </div>
          <div class="lb-field">
            <div class="lb-field-label">Type</div>
            <div class="lb-field-value" x-text="lightbox ? extToType(lightbox.ext) : ''"></div>
          </div>
          <div class="lb-field">
            <div class="lb-field-label">Size</div>
            <div class="lb-field-value" x-text="lightbox ? lightbox.size : ''"></div>
          </div>
          <div class="lb-field">
            <div class="lb-field-label">Location</div>
            <div class="lb-field-value mono" x-text="lightbox ? lightbox.dir : ''"></div>
          </div>
          <div class="lb-field">
            <div class="lb-field-label">Modified</div>
            <div class="lb-field-value" x-text="lightbox ? lightbox.time : ''"></div>
          </div>
        </div>
      </aside>
    </div>
  </div>`

// homeGraphSectionHTML mirrors graph.html's .graph-main block (canvas, zoom
// controls, node info panel, loading/empty states) verbatim — everything
// except the .graph-index-col sidebar, which is now the sidebar's Knowledge
// group (see home.html) instead of living inside the swappable pane. Used by
// homeSectionFullHTML's "full" template when View == "graph".
const homeGraphSectionHTML = `<div class="graph-main">
  <div style="position:relative;flex:1;display:flex;flex-direction:column;overflow:hidden">
    <div id="graph-canvas" style="flex:1;width:100%"></div>

    <div class="graph-zoom-controls">
      <button type="button" class="icon-btn-ghost graph-zoom-btn" title="Zoom in" @click="zoomIn()">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 5v14M5 12h14"/>
        </svg>
      </button>
      <div class="graph-zoom-divider"></div>
      <button type="button" class="icon-btn-ghost graph-zoom-btn" title="Zoom out" @click="zoomOut()">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M5 12h14"/>
        </svg>
      </button>
      <div class="graph-zoom-divider"></div>
      <button type="button" class="icon-btn-ghost graph-zoom-btn" title="Fit all nodes" @click="zoomFit()">
        <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M8 3H5a2 2 0 0 0-2 2v3"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 8V5a2 2 0 0 0-2-2h-3"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16v3a2 2 0 0 0 2 2h3"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M16 21h3a2 2 0 0 0 2-2v-3"/>
        </svg>
      </button>
    </div>

    <div class="graph-node-panel" :class="{ open: !!nodePanel }">
      <div class="graph-node-panel-row">
        <span class="badge badge--sm graph-node-panel-type"
              x-show="nodePanel && nodePanel.entityType"
              x-text="nodePanel ? nodePanel.entityType : ''"></span>
        <span class="graph-node-panel-edges"
              x-text="nodePanel ? (nodePanel.edgeCount + (nodePanel.edgeCount === 1 ? ' link' : ' links')) : ''"></span>
        <button type="button" class="icon-btn-ghost graph-node-panel-close" @click="closeNodePanel()" title="Close">
          <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M18 6 6 18"/>
            <path stroke-linecap="round" d="m6 6 12 12"/>
          </svg>
        </button>
      </div>
      <div class="graph-node-panel-title" x-text="nodePanel ? nodePanel.label : ''"></div>
    </div>

    <div class="graph-loading" x-show="loading" x-cloak>
      <span>Loading graph…</span>
    </div>

    <div class="graph-zero empty-state" x-show="!loading && empty" x-cloak>
      <div class="empty-state-icon">
        <svg fill="none" stroke="currentColor" stroke-width="1.25" viewBox="0 0 48 48">
          <circle cx="24" cy="12" r="4.5"/>
          <circle cx="10" cy="36" r="4.5"/>
          <circle cx="38" cy="36" r="4.5"/>
          <line x1="24" y1="16.5" x2="10.8" y2="31.7" stroke-linecap="round"/>
          <line x1="24" y1="16.5" x2="37.2" y2="31.7" stroke-linecap="round"/>
          <line x1="14.5" y1="36" x2="33.5" y2="36" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="empty-state-title">No knowledge notes yet</div>
      <div class="empty-state-desc">Knowledge notes and their connections will appear here once the compile agent has run on your raw notes.</div>
    </div>
  </div>
</div>`

// homeInboxSectionHTML is the right-pane inbox view: a filter bar (all/
// unread/read + mark-all-read) over a wide-card message list, mirroring the
// visual language of the pinned/folder note list (.home-note-row). Entirely
// client-rendered — home.js fetches /api/inbox once this markup lands in the
// DOM (see the htmx:afterSwap listener) and drives the x-for below.
// Selecting a card opens the read-only message detail, which shares the
// editor's split pane (see contentPaneHTML's inbox-detail-panel) — the two are
// mutually exclusive there, never both visible.
const homeInboxSectionHTML = `<div class="home-inbox-section">
  <div class="home-list-head">
    <span class="home-list-count" title="Unread" x-show="unreadCount > 0" x-text="unreadCount"></span>
    <div class="home-list-head-spacer"></div>
    <div class="seg">
      <button type="button" class="seg-btn" :class="inboxFilter === 'all' ? 'active' : ''" @click="setInboxFilter('all')">All</button>
      <button type="button" class="seg-btn" :class="inboxFilter === 'unread' ? 'active' : ''" @click="setInboxFilter('unread')">Unread</button>
      <button type="button" class="seg-btn" :class="inboxFilter === 'read' ? 'active' : ''" @click="setInboxFilter('read')">Read</button>
    </div>
    <button type="button" class="icon-btn home-inbox-mark-all-btn" title="Mark all read" x-show="unreadCount > 0" @click="markAllInboxRead()">` + svgCheck + `</button>
  </div>
  <div class="home-inbox-list" id="home-inbox-list" @scroll="onInboxListScroll($event)">
    <template x-if="!inboxLoading && visibleInboxMessages().length === 0">
      <div class="home-list-empty" x-text="inboxFilter === 'unread' ? 'All caught up — no unread messages.' : (inboxFilter === 'read' ? 'No read messages yet.' : 'No messages yet.')"></div>
    </template>
    <template x-for="m in visibleInboxMessages()" :key="m.id">
      <div class="list-card list-card--clickable home-list-card home-inbox-card" :class="{ 'is-unread': !m.isRead }" @click="selectInboxMessage(m)">
        <div class="dot dot--fg home-inbox-unread-dot"></div>
        <div class="home-inbox-card-content">
          <div class="home-inbox-card-top">
            <span class="home-inbox-card-title" x-text="m.title || m.source"></span>
            <span class="home-inbox-card-time" x-text="relTime(m.createdAt)"></span>
          </div>
          <div class="home-inbox-card-snippet" x-text="m.body"></div>
        </div>
      </div>
    </template>
    <div class="home-list-empty" x-show="inboxLoadingMore" style="padding:1rem 0">Loading more…</div>
  </div>
</div>`

// homeChatSectionHTML is the chat pane shell. Messages, empty state, and
// composer chrome are filled by chat_view.js; the kernel in chat_kernel.js
// owns the transcript and keeps a run alive across HTMX swaps.
const homeChatSectionHTML = `<div class="chat-main" id="chat-root">

  <div class="home-list-head">
    <span class="agent-bot-bar-desc" id="chat-bot-desc" style="display:none"></span>
    <div class="home-list-head-spacer"></div>
    <div class="seg" id="chat-conv-seg" style="display:none">
      <button class="seg-btn" type="button" data-conv="chat">Chat</button>
      <button class="seg-btn" type="button" data-conv="trigger">Trigger</button>
    </div>
    <button class="new-chat-btn" id="chat-new-btn" type="button" title="New chat" disabled>
      <svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14M5 12h14" />
      </svg>
    </button>
  </div>

  <div class="chat-scroll" id="chat-scroll">
    <div class="chat-scroll-inner" id="chat-thread"></div>
  </div>

  <button type="button" class="chat-scroll-bottom-btn" id="chat-jump" title="Scroll to latest" style="display:none">
    <svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14M5 12l7 7 7-7" />
    </svg>
  </button>

  <div class="chat-input-outer" id="chat-input-outer">
    <div class="chat-ac-wrap">
      <ul class="chat-path-ac" id="chat-path-ac" role="listbox" aria-expanded="false" hidden></ul>
      <div class="chat-input-card" id="chat-input-card">
        <div class="chat-input-hint" id="chat-input-hint" aria-hidden="true">
          <span class="chat-hint-name" id="chat-hint-name"></span>
        </div>
        <textarea class="chat-textarea" id="chat-textarea" rows="1" placeholder=""></textarea>
        <span class="chat-shortcut-hint" id="chat-shortcut-hint"></span>
        <button type="button" class="chat-send-circle" id="chat-send" disabled>
          <svg fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 19V5M5 12l7-7 7 7" />
          </svg>
        </button>
        <button type="button" class="chat-stop-circle" id="chat-stop" style="display:none">
          <svg fill="currentColor" viewBox="0 0 24 24">
            <rect x="5" y="5" width="14" height="14" rx="2" />
          </svg>
        </button>
      </div>
    </div>
  </div>

</div>`

// homeChatToastHTML mirrors agent_chat.html's run-completion toast — a
// position:fixed element, so like homeImagesLightboxHTML it lives once at
// the page level rather than inside the swappable
// #home-list-pane: a background chat run can finish while a different
// sidebar section is showing, and the toast should still surface.
const homeChatToastHTML = `
  <div class="run-toast" :class="toastKind === 'ok' ? 'run-toast-ok' : 'run-toast-err'" x-show="toastVisible" x-transition
    style="display:none">
    <span x-text="toastText"></span>
    <button class="run-toast-dismiss" @click="toastVisible = false" type="button" title="Dismiss">
      <svg fill="none" stroke="currentColor" stroke-width="1.7" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6" />
        <path stroke-linecap="round" stroke-linejoin="round" d="m6 6 12 12" />
      </svg>
    </button>
  </div>`

func (vh *ViewHandler) noteItemsFromNotes(notes []storage.Note) []noteItem {
	items := make([]noteItem, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteToItem(n))
	}
	attachCovers(vh.vault, items)
	return items
}

var homePageHTML = `<!DOCTYPE html>
<html lang="en">
` + headHTML(headOpts{title: "Home — Vaultr", withFonts: true, withTW: true, withAlpine: true, withHTMX: true}) + `
  <script src="/static/vendor/cytoscape.min.js"></script>
  <script src="/static/vendor/layout-base.js"></script>
  <script src="/static/vendor/cose-base.js"></script>
  <script src="/static/vendor/cytoscape-fcose.min.js"></script>
  <script src="/static/vendor/d3-cloud.min.js"></script>
  <script src="/static/vendor/marked.min.js"></script>
  <script src="/static/vendor/dompurify.min.js"></script>
  <script>
  if (typeof marked !== 'undefined') { marked.setOptions({ gfm: true, breaks: true }); }
  </script>
  <style>
` + appTokensCSS + `
` + infoDialogCSS + baseCSS + cselectCSS + homeCSS + imagesCSS + graphCSS + agentChatCSS + contentPaneCSS + noteSharedCSS + noteEditorCSS + shortsCSS + searchOverlayStyles + confirmDialogCSS + settingsModalCSS + frontmatterDialogCSS + `
  </style>
</head>
<body x-data="homeCtrl()" @vaultr:insert-path.window="insertPath($event)">
` + searchOnlyOverlayHTML + confirmDialogHTML + infoDialogHTML + frontmatterDialogHTML + settingsModalHTML() + homeImagesLightboxHTML + homeChatToastHTML + `
  <div class="home-container">
` + homeMainHTML + contentPaneHTML + `
    </div><!-- /.home-main -->
  </div><!-- /.home-shell -->
</div><!-- /.home-container -->

  <div id="graph-tooltip" class="graph-tooltip"></div>

  <script>
  document.addEventListener('alpine:init', () => {
` + alpineStoresScript + `
  });

` + keysJS + pathAcScript + chatReduceJS + chatTextJS + chatKernelJS + chatViewJS + contentPaneScript + searchOverlayScript + confirmDialogJS + infoDialogJS + frontmatterDialogJS + settingsCtrlJS + homeJS + `
  </script>
` + noteSharedJS + `
</body>
</html>`

var homePageTemplate = template.Must(template.New("home").Funcs(homeTemplateFuncs).Parse(homePageHTML))

// HomeRefresh handles GET /home/refresh — returns HTMX OOB fragments for the
// sidebar's counts + folder list, without a full page reload.
// It does not touch the active note list — the client re-requests that
// separately via homeCtrl.reloadActiveSection (see home.js), since only the
// browser knows which sidebar item is currently selected.
func (vh *ViewHandler) HomeRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}

	folders, err := vh.vault.ListDirs()
	if err != nil {
		http.Error(w, "home/refresh: list dirs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	indexNotes := vh.listIndexItems()

	data := homePageData{
		Folders:        folders,
		IndexNotes:     indexNotes,
		KnowledgeCount: vh.knowledgeCount(),
	}

	var buf bytes.Buffer
	if err := homeRefreshTemplate.Execute(&buf, data); err != nil {
		http.Error(w, fmt.Sprintf("home/refresh: render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

var homeRefreshTemplate = template.Must(template.New("home-refresh").Funcs(homeTemplateFuncs).Parse(`<div id="home-side-folders-body" class="home-side-children" :class="{'is-open': foldersOpen}" x-cloak hx-swap-oob="true">
  <div class="home-side-children-inner">
    {{range .Folders}}
    <button type="button" class="side-nav-item home-side-child home-drop-target" :class="{'is-active': activeKey === 'dir:{{.Dir}}'}"
            data-drop-dir="{{.Dir}}"
            @click="selectFolder('{{.Dir}}','/home/section?type=folder&path={{encdir .Dir}}')">
      <svg class="home-side-child-icon" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"
           stroke-linejoin="round" viewBox="0 0 24 24">
        <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
      </svg>
      <span class="home-side-child-name">{{folderLabel .Dir}}</span>
    </button>
    {{end}}
    {{if not .Folders}}<div class="home-side-empty">No folders</div>{{end}}
  </div>
</div>
<div id="home-side-knowledge-body" class="home-side-children" :class="{'is-open': knowledgeOpen}" x-cloak hx-swap-oob="true">
  <div class="home-side-children-inner">
    <button type="button" class="side-nav-item home-side-child" :class="{'is-active': activeKey === 'knowledge:'}"
            @click="selectKnowledgeIndex('')">
      <span class="home-side-child-name">All</span>
      <span class="home-side-count">{{.KnowledgeCount}}</span>
    </button>
    {{range .IndexNotes}}
    <button type="button" class="side-nav-item home-side-child" :class="{'is-active': activeKey === 'knowledge:{{.Path}}'}"
            @click="selectKnowledgeIndex('{{.Path}}')">
      <span class="home-side-child-name">{{label .}}</span>
      <span class="home-side-count">{{.DepCount}}</span>
    </button>
    {{end}}
  </div>
</div>
`))
