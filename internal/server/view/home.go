package view

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

const homeListPageSize = 20

type homePageData struct {
	Folders        []storage.DirSummary
	IndexNotes     []noteItem
	KnowledgeCount int // total notes under the knowledge dir — the sidebar's "All" count
	SectionHTML    template.HTML
	Boot           homeBoot
}

// homeBoot is client state the server already knows. The page is rendered
// from it and homeCtrl starts from it (#home-boot), so first paint already
// matches what Alpine will show.
type homeBoot struct {
	UnreadCount int       `json:"unreadCount"`
	Vault       string    `json:"vault"`
	Bots        []bootBot `json:"bots"`
}

// bootBot is what a sidebar bot row draws; /api/mates fills in the rest.
type bootBot struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (vh *ViewHandler) homeBoot() homeBoot {
	b := homeBoot{Vault: vh.vault.Root(), Bots: []bootBot{}}
	if vh.inbox != nil {
		b.UnreadCount, _ = vh.inbox.UnreadCount()
	}
	if vh.mates != nil {
		list, _ := vh.mates.ListMates()
		for _, m := range list {
			if m.Enabled {
				b.Bots = append(b.Bots, bootBot{ID: m.ID, Name: m.Name, Color: m.Color})
			}
		}
	}
	return b
}

func (d homePageData) HasFolders() bool   { return len(d.Folders) > 0 }
func (d homePageData) HasKnowledge() bool { return d.KnowledgeCount > 0 || len(d.IndexNotes) > 0 }

// homeSectionData drives the right-hand note list — the content shown for
// whichever sidebar item (Pinned / a folder) is active. Shorts is
// a different "view kind" (the rendered shorts stream, not a note list) and
// is rendered separately by renderHomeShortsSection; see renderHomeSectionHTML.
type homeSectionData struct {
	Title   string
	Count   int
	Items   []noteItem
	NextURL string // hx-get URL for the load-more sentinel; empty = no more pages
	Empty   emptyState
	// ShowGraphOption/View: Knowledge only. Graph is just a third way to
	// look at the same knowledge notes (alongside list/grid), picked via the
	// seg control — see selectKnowledgeIndex/setListView in home.js.
	ShowGraphOption bool
	View            string // "list" (default) | "grid" | "graph"
}

// emptyState is what a section shows when it has nothing yet.
type emptyState struct {
	Icon, Title, Desc string
	NewNote           bool
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
		Boot:           vh.homeBoot(),
	}

	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "page_home.html", data); err != nil {
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
		data.Empty = emptyState{Icon: "folders", Title: "This folder is empty", Desc: "Drag notes onto it in the sidebar to move them here."}
		if dirPath == "/" {
			data.Empty = emptyState{Icon: "folders", Title: "No notes yet", Desc: "Just write. Hylo learns what matters to you from your notes.", NewNote: true}
		}
		data.NextURL = homeSectionMoreURL("folder", dirPath, nextNs)
		if !itemsOnly {
			data.Count = vh.dirNoteCount(dirPath)
		}
	case "memory":
		dirPath := storage.MemoryDir
		var nextNs int64
		data.Items, nextNs = vh.listDirNoteItems(dirPath, beforeNs, homeListPageSize)
		data.Title = "Memory"
		data.Empty = emptyState{Icon: "memory", Title: "No memories yet", Desc: "Hylo learns who you are from your notes and keeps it here."}
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
		data.Empty = emptyState{Icon: "knowledge", Title: "No knowledge yet", Desc: "Hylo compiles your notes into linked knowledge. Compile a note, or clip a page into Web Clips."}

		indexParam := strings.TrimSpace(r.URL.Query().Get("index"))
		if indexParam == "" {
			dirPath := storage.KnowledgeDir
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
			data.Empty = emptyState{Icon: "knowledge", Title: "Nothing in this category yet"}
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
	case "recent":
		notes, err := vh.vault.ListRecentNotes(time.Now().AddDate(0, 0, -7))
		if err != nil {
			return data, err
		}
		data.Items = vh.noteItemsFromNotes(notes)
		data.Title = "Recent"
		data.Empty = emptyState{Icon: "recent", Title: "Nothing this week", Desc: "Notes you edit over the last 7 days show up here."}
		data.Count = len(data.Items)
	default: // "pinned"
		pinned, err := vh.vault.ListPinnedNotes()
		if err != nil {
			return data, err
		}
		data.Items = vh.noteItemsFromNotes(pinned)
		data.Title = "Pinned"
		data.Empty = emptyState{Icon: "star", Title: "No pinned notes", Desc: "Pin the notes you come back to often to keep them here."}
		data.Count = len(data.Items)
	}
	return data, nil
}

// knowledgeCount returns the total note count under the knowledge dir — the
// sidebar's "All" child count, and the same number homeSectionData's
// unfiltered "knowledge" case uses for its list-head pill.
func (vh *ViewHandler) knowledgeCount() int {
	return vh.dirNoteCount(storage.KnowledgeDir)
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

// renderHomeSectionHTML resolves a /home or /home/section request into the
// #home-list-pane contents for whichever sidebar item is active. This is the
// dispatch point between the right pane's different "view kinds": today a
// generic note list (pinned/folder), the rendered Shorts stream, the image
// gallery grid, the tag cloud/picker, the knowledge graph canvas, the inbox
// message list, or the agent chat panel; more kinds can be added here as new
// cases without touching the sidebar's swap mechanism (see home.js go()).
func (vh *ViewHandler) renderHomeSectionHTML(r *http.Request) (template.HTML, error) {
	switch r.URL.Query().Get("type") {
	case "shorts":
		return vh.renderHomeShortsSection(r)
	case "images":
		return vh.renderHomeImagesSection(r)
	case "tags":
		return vh.renderHomeTagsSection(r)
	case "inbox", "":
		// The message list is client-rendered (home.js fetches /api/inbox once
		// #home-inbox-list lands — see __hyloSectionArrived); only the head
		// renders the unread count.
		return renderHTML("home_inbox_section.html", homePageData{Boot: vh.homeBoot()})
	case "chat":
		// Static shell. home.js attaches the chat kernel once #chat-root
		// lands in the DOM (__hyloSectionArrived); the markup itself never varies.
		return renderHTML("home_chat_section.html", nil)
	}
	data, err := vh.homeSectionData(r, false)
	if err != nil {
		return "", err
	}
	return renderHTML("full", data)
}

// renderHomeShortsSection renders the Shorts view: a header (title + a
// "jump to month" select), a composer, and the date-grouped entry feed
// (shorts.go's loadStreamGroups/loadMonthsWithEntries back both). Pagination
// ("Load earlier") and the month select both stay in place inside
// #home-list-pane, rather than navigating anywhere — unlike the
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

	tag := util.NormalizeTag(r.URL.Query().Get("tag"))
	groups, cursor, hasMore, err := vh.loadStreamGroups(before, tag, 50)
	if err != nil {
		return "", err
	}

	data := shortsStreamPageData{
		Groups:           groups,
		Cursor:           cursor,
		HasMore:          hasMore,
		Tag:              tag,
		Tags:             vh.shortsTagRow(tag),
		Months:           vh.loadMonthsWithEntries(activeYM),
		ActiveMonthLabel: activeMonthLabel,
	}

	return renderHTML("home_shorts_section.html", data)
}

// renderHomeImagesSection renders the image gallery grid (toolbar, thumbnail
// grid, select/delete). Pagination goes through /images/grid, whose #img-grid
// target and sentinel markup match here.
// The lightbox overlay this grid opens lives once at the page level (see
// page_home.html) rather than inside this swappable fragment.
func (vh *ViewHandler) renderHomeImagesSection(r *http.Request) (template.HTML, error) {
	imgs, err := vh.vault.ListImages(0, imagesPageSize+1)
	if err != nil {
		return "", err
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

	count, err := vh.vault.CountImages()
	if err != nil {
		return "", err
	}

	return renderHTML("home_images_section.html", imagesGridData{Images: items, NextNs: nextNs, Count: count})
}

// tagWallMaxTags caps how many distinct tags the wall/picker ever fetch —
// generous for "a few hundred tags" without dragging in a vault's entire
// long tail (TagDistribution's own default is 500; a vault with more than
// this many distinct tags still works, it just won't show the extreme tail
// in the wall).
const tagWallMaxTags = 2000

// tagNotesLimit caps how many notes a single tag's filtered list can show.
// Unlike folder/knowledge lists this has no "load more" pagination — a limit
// this high is just a safety valve, not a real page size.
const tagNotesLimit = 5000

// tagItem is one word in the Tags wall's cloud, or one option in the
// filterable tag picker dropdown shown above a selected tag's note list (see
// homeTagsSectionHTML). JSON tags matter: the wall passes the whole slice to
// the client as JSON (see the "tagsJSON" template func below) for d3-cloud
// to lay out — see __hyloRenderTagCloud in home.js.
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
	Tags      []tagItem
	Selected  string
	Items     []noteItem
	Count     int
	NextURL   string
	Empty     emptyState
	WallEmpty emptyState
}

// renderHomeTagsSection renders the Tags wall (no ?tag=) or, once a tag is
// picked, that tag's filtered note list with a one-line head (current tag +
// count + a single "switch tag" dropdown) instead of listing every tag —
// switching never requires returning to the wall first (see selectTag in
// home.js). Both states share one fetch of the full tag distribution, since
// the picker dropdown reuses the wall's own tag/count data.
func (vh *ViewHandler) renderHomeTagsSection(r *http.Request) (template.HTML, error) {
	data := homeTagsData{
		Empty:     emptyState{Icon: "tags", Title: "No notes with this tag"},
		WallEmpty: emptyState{Icon: "tags", Title: "No tags yet", Desc: "Add #tags to your notes to group them here."},
	}

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

	return renderHTML("tags", data)
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
	if err := views.ExecuteTemplate(w, "rows", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}

// todoRingRadius matches the <circle r="..."> in the "rows" template's
// .note-badge--todo markup — keep the two in sync if that ever changes.
const todoRingRadius = 8.0

var todoRingCircumference = 2 * math.Pi * todoRingRadius

func (vh *ViewHandler) noteItemsFromNotes(notes []storage.Note) []noteItem {
	items := make([]noteItem, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteToItem(n))
	}
	attachCovers(vh.vault, items)
	return items
}

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
	if err := views.ExecuteTemplate(&buf, "home_refresh.html", data); err != nil {
		http.Error(w, fmt.Sprintf("home/refresh: render: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
