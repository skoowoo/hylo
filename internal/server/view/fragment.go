package view

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// NoteFragment handles GET /notes/fragment?path=... or ?name=...
// Returns a bare HTML fragment (no full-page shell) for loading into the
// reading content pane via HTMX.
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
	CoverTitle  string
	Dir         string
	Frontmatter util.Frontmatter
	Content     template.HTML
}

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
	raw, err := io.ReadAll(io.LimitReader(rc, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read note: %w", err)
	}
	if !util.IsValidText(raw) {
		return nil, storage.ErrBinaryContent
	}
	return vh.fragmentHTMLFromRaw(p, raw)
}

// fragmentHTMLFromRaw renders the note fragment HTML from already-read source bytes.
func (vh *ViewHandler) fragmentHTMLFromRaw(p storage.Path, raw []byte) ([]byte, error) {
	fm, body := util.ParseFrontmatter(raw)
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
	data := noteFragData{
		CoverTitle:  coverTitle,
		Dir:         p.Dir(),
		Frontmatter: fm,
		Content:     template.HTML(fragment), //nolint:gosec // fragment is server-generated markdown HTML
	}
	var buf bytes.Buffer
	if err := noteFragTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return buf.Bytes(), nil
}

var noteFragTmpl = template.Must(template.New("notefrag").Parse(noteFragHTML))

const noteFragHTML = `<header class="frag-cover">
  {{- if ne .Dir "/"}}
  <p class="cover-dir frag-dir">{{.Dir}}</p>
  {{- end}}
  <h1 class="frag-title" id="frag-cover-title">{{.CoverTitle}}</h1>
</header>

{{- if .Frontmatter.HasMeta}}
<div class="frag-fm-wrap">
  <details class="fm-details" open>
    <summary class="fm-summary">
      <span class="fm-arrow">▶</span>
      metadata
    </summary>
    <dl class="fm-grid">
      {{- range $i, $v := .Frontmatter.All}}
      <dt class="fm-key{{if ge $i 3}} fm-overflow{{end}}">{{$v.Key}}</dt>
      <dd class="fm-val{{if ge $i 3}} fm-overflow{{end}}">
        {{- if $v.IsList}}
        {{- range $v.List}}<span class="badge fm-tag">{{.}}</span>{{end}}
        {{- else if $v.IsURL}}
        <span class="fm-val-text" title="{{$v.Value}}"><a href="{{$v.Value}}" target="_blank" rel="noopener noreferrer">{{$v.Value}}</a></span>
        {{- else if $v.Value}}
        <span class="fm-val-text" title="{{$v.Value}}">{{$v.Value}}</span>
        {{- end}}
      </dd>
      {{- end}}
    </dl>
    {{- if gt (len .Frontmatter.All) 3}}
    <button class="fm-toggle" onclick="fmToggleGrid(this)"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m7 6 5 5 5-5"/><path d="m7 13 5 5 5-5"/></svg></button>
    {{- end}}
  </details>
</div>
{{- end}}

<article class="prose frag-article">{{.Content}}</article>
`
