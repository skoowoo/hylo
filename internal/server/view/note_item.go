package view

import (
	"net/url"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

// noteItem is the shared per-note view model rendered by home's note list,
// folder view, and Knowledge sidebar group.
type noteItem struct {
	Name        string
	Title       string // LLM-generated title; non-empty for distill notes
	Dir         string
	Preview     string // short content excerpt (storage.Note.Preview.Text); empty when not yet computed
	Path        string
	UpdatedAt   string
	URL         string // full view page URL
	FragmentURL string // content-pane fragment URL
	CursorNs    int64  // Unix nanoseconds for pagination cursor
	IsKnowledge bool   // true for knowledge notes; drives data-knowledge on the read button
	IsIndex     bool   // true for index notes (origin = "plugin:index")
	Pinned      bool   // true when the note is pinned by the user
	CanCompile  bool   // true when this note is eligible to be compiled (raw, compile_count==0)
	IsCompiled  bool   // true when raw note has been compiled at least once (compile_count>0, not knowledge/index)
	DepCount    int    // number of knowledge deps; set on index cards
	Cover       string // image filename only; empty = no cover
	TodoTotal   int    // GFM task-list checkboxes anywhere in the note; 0 = no checklist
	TodoDone    int    // how many of TodoTotal are checked
}

// noteToItem converts a storage.Note to a noteItem for template rendering.
func noteToItem(n storage.Note) noteItem {
	isKnowledge := n.Kind == storage.KindKnowledge
	isIndex := n.Kind == storage.KindIndex
	q := url.Values{}
	q.Set("path", n.PathString())
	return noteItem{
		Name:        strings.TrimSuffix(n.Name, ".md"),
		Title:       n.Title,
		Dir:         n.Dir,
		Preview:     n.Preview.Text,
		TodoTotal:   n.Preview.TodoTotal,
		TodoDone:    n.Preview.TodoDone,
		Path:        n.PathString(),
		UpdatedAt:   formatRelativeTime(n.UpdatedAt),
		Pinned:      n.Pinned,
		IsKnowledge: isKnowledge,
		IsIndex:     isIndex,
		CanCompile:  !isKnowledge && !isIndex && n.CompileCount == 0,
		IsCompiled:  !isKnowledge && !isIndex && n.CompileCount > 0,
		URL:         "/notes?" + q.Encode(),
		FragmentURL: "/notes/fragment?" + q.Encode(),
		CursorNs:    n.UpdatedAt.UnixNano(),
	}
}

// listIndexItems returns all index notes, newest first, with DepCount filled
// in. Used by home.go's Knowledge sidebar group.
func (vh *ViewHandler) listIndexItems() []noteItem {
	notes, err := vh.vault.ListAllNotes(storage.ListOptions{
		OnlyKinds:  []storage.Kind{storage.KindIndex},
		SortByTime: true,
	})
	if err != nil {
		return nil
	}
	depCounts, _ := vh.vault.GetAllIndexDepCounts()
	items := make([]noteItem, 0, len(notes))
	for _, n := range notes {
		item := noteToItem(n)
		if depCounts != nil {
			item.DepCount = depCounts[n.PathString()]
		}
		items = append(items, item)
	}
	attachCovers(vh.vault, items)
	return items
}

// listDirNoteItems returns notes in dir, newest first, paginated by beforeNs.
// Used by home.go's folder view.
func (vh *ViewHandler) listDirNoteItems(dir string, beforeNs int64, limit int) ([]noteItem, int64) {
	opts := storage.ListOptions{
		SortByTime: true,
		Limit:      limit + 1,
	}
	if beforeNs > 0 {
		opts.Before = time.Unix(0, beforeNs)
	}

	notes, err := vh.vault.ListDirNotes(dir, opts)
	if err != nil {
		return nil, 0
	}

	hasMore := len(notes) > limit
	if hasMore {
		notes = notes[:limit]
	}

	items := make([]noteItem, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteToItem(n))
	}
	attachCovers(vh.vault, items)

	var nextNs int64
	if hasMore && len(items) > 0 {
		nextNs = items[len(items)-1].CursorNs
	}
	return items, nextNs
}

func attachCovers(vault *storage.Vault, items []noteItem) {
	if vault == nil || len(items) == 0 {
		return
	}
	paths := make([]storage.Path, 0, len(items))
	for _, it := range items {
		if p, ok := storage.ParsePath(it.Path); ok {
			paths = append(paths, p)
		}
	}
	covers, err := vault.NoteAssetFilenames(paths, storage.AssetKindCover)
	if err != nil {
		return
	}
	for i := range items {
		items[i].Cover = covers[items[i].Path]
	}
}
