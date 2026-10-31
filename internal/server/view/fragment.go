package view

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// NoteFragment handles GET /notes/fragment?path=... or ?name=...
// Returns a bare HTML fragment of the note's beginning for the search preview.
func (vh *ViewHandler) NoteFragment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !htmxOnly(w, r, "/home") {
		return
	}
	p := r.URL.Query().Get("path")
	name := r.URL.Query().Get("name")
	switch {
	case p != "" && name != "":
		http.Error(w, `specify either "path" or "name", not both`, http.StatusBadRequest)
	case p != "":
		vh.fragmentByPath(w, p)
	case name != "":
		vh.fragmentByName(w, name)
	default:
		http.Error(w, `missing required parameter: "path" or "name"`, http.StatusBadRequest)
	}
}

func (vh *ViewHandler) fragmentByPath(w http.ResponseWriter, pathStr string) {
	p, ok := storage.ParsePath(pathStr)
	if !ok {
		http.Error(w, `path must be absolute (start with "/")`, http.StatusBadRequest)
		return
	}
	p, _ = storage.ParsePath(storage.JoinPath(p.Dir(), ensureMarkdownName(p.Base())))
	vh.renderFragment(w, p)
}

func (vh *ViewHandler) fragmentByName(w http.ResponseWriter, name string) {
	if strings.ContainsAny(name, "/\\") {
		http.Error(w, `"name" must be a filename only`, http.StatusBadRequest)
		return
	}
	name = ensureMarkdownName(name)
	notes, err := vh.vault.GetNotesByName(name)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	if len(notes) == 0 {
		http.Error(w, storage.ErrNotFound.Error(), http.StatusNotFound)
		return
	}
	vh.renderFragment(w, notes[0].Path())
}

type noteFragData struct {
	CoverTitle string
	Dir        string
	UpdatedAt  string
	Content    template.HTML
	Truncated  bool
}

// previewBytes caps how much of a note the preview reads and renders; the
// pane shows a screenful, so a long note costs no more than a short one.
const previewBytes = 8 << 10

func (vh *ViewHandler) renderFragment(w http.ResponseWriter, p storage.Path) {
	html, err := vh.fragmentHTML(p)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

// fragmentHTML reads the note at p and returns the rendered HTML fragment bytes.
func (vh *ViewHandler) fragmentHTML(p storage.Path) ([]byte, error) {
	if !util.IsMarkdownPath(p.Base()) {
		return nil, fmt.Errorf("only markdown notes can be viewed")
	}
	rc, err := vh.vault.ReadNoteStream(p)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, previewBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read note: %w", err)
	}
	truncated := len(raw) > previewBytes
	if truncated {
		raw = previewCut(raw[:previewBytes])
	}
	if !util.IsValidText(raw) {
		return nil, storage.ErrBinaryContent
	}
	return vh.fragmentHTMLFromRaw(p, raw, truncated)
}

// previewCut ends a prefix at its last block boundary outside a code fence,
// so the preview never stops mid-paragraph or splits a rune.
func previewCut(b []byte) []byte {
	lastBlank, lastNL, inFence := -1, -1, false
	for i := 0; i < len(b); {
		end := bytes.IndexByte(b[i:], '\n')
		if end < 0 {
			break
		}
		line := bytes.TrimSpace(b[i : i+end])
		if bytes.HasPrefix(line, []byte("```")) || bytes.HasPrefix(line, []byte("~~~")) {
			inFence = !inFence
		} else if len(line) == 0 && !inFence {
			lastBlank = i
		}
		i += end + 1
		lastNL = i
	}
	switch {
	case lastBlank > 0:
		return b[:lastBlank]
	case lastNL > 0:
		return b[:lastNL]
	}
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b
}

// fragmentHTMLFromRaw renders the note fragment HTML from already-read source bytes.
func (vh *ViewHandler) fragmentHTMLFromRaw(p storage.Path, raw []byte, truncated bool) ([]byte, error) {
	_, body := util.ParseFrontmatter(raw)
	coverTitle := util.ExtractMarkdownH1(body)
	bodyForRender := body
	if coverTitle != "" {
		bodyForRender = util.StripFirstH1(body)
	} else {
		coverTitle = strings.TrimSuffix(p.Base(), ".md")
	}
	fragment, err := util.MarkdownToHTMLFragmentChecked(bodyForRender, vh.wikilinksExistFunc(body))
	if err != nil {
		return nil, err
	}
	updatedAt := ""
	if notes, err := vh.vault.GetNotesByPaths([]storage.Path{p}); err == nil && len(notes) > 0 && !notes[0].UpdatedAt.IsZero() {
		updatedAt = formatRelativeTime(notes[0].UpdatedAt)
	}
	data := noteFragData{
		CoverTitle: coverTitle,
		Dir:        p.Dir(),
		UpdatedAt:  updatedAt,
		Content:    template.HTML(fragment), //nolint:gosec // fragment is server-generated markdown HTML
		Truncated:  truncated,
	}
	var buf bytes.Buffer
	if err := views.ExecuteTemplate(&buf, "note_fragment.html", data); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return buf.Bytes(), nil
}
