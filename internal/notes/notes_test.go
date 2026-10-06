package notes

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/storage"
)

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("error = %v, want containing %q", err, substr)
	}
}

func TestTimeFilterBounds(t *testing.T) {
	if _, _, err := (TimeFilter{Latest: 3, Start: "2026-01-01"}).Bounds(); err == nil {
		t.Error("latest with start should fail")
	}
	_, _, err := TimeFilter{Start: "01/02/2026"}.Bounds()
	wantErr(t, err, "invalid start date")
	_, _, err = TimeFilter{End: "nope"}.Bounds()
	wantErr(t, err, "invalid end date")

	after, before, err := TimeFilter{Start: "2026-01-01", End: "2026-02-01"}.Bounds()
	if err != nil || after.Format("2006-01-02") != "2026-01-01" || before.Format("2006-01-02") != "2026-02-01" {
		t.Errorf("bounds = %v %v %v", after, before, err)
	}
	after, before, _ = TimeFilter{Latest: 7}.Bounds()
	if after.IsZero() || !before.IsZero() {
		t.Errorf("latest bounds = %v %v", after, before)
	}
}

func TestListQueryOptions(t *testing.T) {
	cases := []struct {
		q       ListQuery
		only    []storage.Kind
		exclude int
		err     string
	}{
		{ListQuery{}, nil, 0, ""},
		{ListQuery{Kind: "raw"}, nil, 3, ""},
		{ListQuery{Kind: "short"}, []storage.Kind{storage.KindShort}, 0, ""},
		{ListQuery{Kind: "bogus"}, nil, 0, "must be raw, short, knowledge, or index"},
		{ListQuery{Knowledge: true}, []storage.Kind{storage.KindKnowledge, storage.KindIndex}, 0, ""},
		{ListQuery{Knowledge: true, Kind: "index"}, []storage.Kind{storage.KindIndex}, 0, ""},
		{ListQuery{Knowledge: true, Kind: "raw"}, nil, 0, "must be knowledge or index"},
	}
	for _, c := range cases {
		opts, err := c.q.options()
		if c.err != "" {
			wantErr(t, err, c.err)
			continue
		}
		if err != nil || len(opts.OnlyKinds) != len(c.only) || len(opts.ExcludeKinds) != c.exclude || !opts.SortByTime {
			t.Errorf("%+v: opts=%+v err=%v", c.q, opts, err)
		}
	}
}

func TestSegment(t *testing.T) {
	text := "a\nb\nc\nd\n"
	cases := []struct {
		spec  SegmentSpec
		want  string
		total int
	}{
		{SegmentSpec{Head: 2}, "a\nb", 4},
		{SegmentSpec{Tail: 2}, "c\nd", 4},
		{SegmentSpec{Start: 2, End: 3}, "b\nc", 4},
		{SegmentSpec{Head: 99}, "a\nb\nc\nd", 4},
		{SegmentSpec{Tail: 99}, "a\nb\nc\nd", 4},
		{SegmentSpec{Start: 9, End: 10}, "", 4},
	}
	for _, c := range cases {
		got, total := Segment(text, c.spec)
		if strings.Join(got, "\n") != c.want || total != c.total {
			t.Errorf("%+v: got %q total %d", c.spec, got, total)
		}
	}
	if lines, total := Segment("", SegmentSpec{Head: 1}); len(lines) != 0 || total != 0 {
		t.Errorf("empty text: %v %d", lines, total)
	}
	if lines, _ := Segment("x\n\ny\n", SegmentSpec{Start: 2, End: 2}); len(lines) != 1 || lines[0] != "" {
		t.Errorf("blank line in range must be returned as a line, got %q", lines)
	}
}

func TestSegmentSpecValidate(t *testing.T) {
	cases := []struct {
		spec SegmentSpec
		err  string
	}{
		{SegmentSpec{}, "specify one of"},
		{SegmentSpec{Head: 1, Tail: 1}, "mutually exclusive"},
		{SegmentSpec{Head: 1, Start: 1, End: 2}, "mutually exclusive"},
		{SegmentSpec{Start: 2}, "used together"},
		{SegmentSpec{End: 2}, "used together"},
		{SegmentSpec{Start: 5, End: 2}, "must not be greater"},
		{SegmentSpec{Head: 3}, ""},
		{SegmentSpec{Start: 2, End: 2}, ""},
	}
	for _, c := range cases {
		err := c.spec.Validate()
		if c.err == "" && err != nil {
			t.Errorf("%+v: %v", c.spec, err)
		} else if c.err != "" {
			wantErr(t, err, c.err)
		}
	}
}

func TestSectionAndTags(t *testing.T) {
	src := []byte("---\ntags: [a, b]\n---\n# T\n\n## Goals\n\nship\n\n## Other\n\nx\n")
	sec, err := Section(src, "goals")
	if err != nil || !strings.Contains(sec, "ship") || strings.Contains(sec, "Other") {
		t.Errorf("section = %q err=%v", sec, err)
	}
	_, err = Section(src, "missing")
	wantErr(t, err, "no heading matching")

	hasFM, tags := Tags(src)
	if !hasFM || len(tags) != 2 {
		t.Errorf("tags = %v %v", hasFM, tags)
	}
	if hasFM, tags := Tags([]byte("# plain\n")); hasFM || tags == nil || len(tags) != 0 {
		t.Errorf("no front matter: %v %#v", hasFM, tags)
	}
}

func TestPathValidation(t *testing.T) {
	wantErr(t, CheckAbs("path", "rel.md"), "must be absolute")
	wantErr(t, CheckMarkdownPath("/a.txt"), "not a markdown file")
	wantErr(t, CheckText([]byte{0xff, 0xfe, 0xfd}), "binary")
	if EnsureMarkdownExt("today") != "today.md" || EnsureMarkdownExt("a.markdown") != "a.markdown" {
		t.Error("EnsureMarkdownExt")
	}
	c := client.NewInProcess(http.NotFoundHandler()) // validation must fail before any request
	wantErr(t, func() error { _, err := Move(c, "/a.md", "rel"); return err }(), "new-dir")
	wantErr(t, func() error { _, _, err := Rename(c, "/a.md", "x/y.md"); return err }(), "filename only")
	wantErr(t, Delete(c, "a.md"), "must be absolute")
	wantErr(t, DeleteKnowledge(c, "a.md"), "must be absolute")
	_, err := ShortCreate(c, "", "")
	wantErr(t, err, "must not be empty")
}

// fakeVault serves just enough of the hylo API for the shared operations.
type fakeVault struct {
	notes   map[string]storage.Note // name -> note
	files   map[string]string       // path -> content
	written map[string]string
	hits    []client.SearchResult
}

func (f *fakeVault) handler() http.Handler {
	mux := http.NewServeMux()
	body := func(r *http.Request) map[string]any {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		return m
	}
	mux.HandleFunc("POST /api/notes/resolve", func(w http.ResponseWriter, r *http.Request) {
		name := body(r)["name"].(string)
		var matches []storage.Note
		for _, n := range f.notes {
			if n.Name == name {
				matches = append(matches, n)
			}
		}
		_ = json.NewEncoder(w).Encode(client.NoteResolveResponse{Name: name, Matches: matches, Count: len(matches)})
	})
	mux.HandleFunc("POST /api/vault/stat", func(w http.ResponseWriter, r *http.Request) {
		p := body(r)["path"].(string)
		for _, n := range f.notes {
			if n.PathString() == p {
				_ = json.NewEncoder(w).Encode(n)
				return
			}
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("POST /api/vault/read", func(w http.ResponseWriter, r *http.Request) {
		p, _ := body(r)["path"].(string)
		if c, ok := f.files[p]; ok {
			_, _ = io.WriteString(w, c)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("POST /api/vault/write", func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		f.written[m["path"].(string)] = m["content"].(string)
	})
	mux.HandleFunc("POST /api/search", func(w http.ResponseWriter, r *http.Request) {
		limit := int(body(r)["limit"].(float64))
		hits := f.hits
		if len(hits) > limit {
			hits = hits[:limit]
		}
		_ = json.NewEncoder(w).Encode(client.SearchResponse{Query: "q", Total: len(hits), Results: hits})
	})
	return mux
}

func newFakeVault() (*fakeVault, *client.Client) {
	f := &fakeVault{
		notes: map[string]storage.Note{
			"a":  {Dir: "/x", Name: "dup.md"},
			"b":  {Dir: "/y", Name: "dup.md"},
			"k":  {Dir: "/_knowledge", Name: "unit.md", Kind: storage.KindKnowledge},
			"r":  {Dir: "/", Name: "raw.md"},
			"kk": {Dir: "/_knowledge", Name: "raw.md", Kind: storage.KindKnowledge},
		},
		files:   map[string]string{"/raw.md": "raw body", "/_knowledge/unit.md": "unit body"},
		written: map[string]string{},
	}
	return f, client.NewInProcess(f.handler())
}

func TestResolvePath(t *testing.T) {
	_, c := newFakeVault()

	p, err := ResolvePath(c, "raw", false) // matches /raw.md and /_knowledge/raw.md
	wantErr(t, err, "multiple notes named")
	if !strings.Contains(err.Error(), "/raw.md") || p != "" {
		t.Errorf("err = %v", err)
	}
	if p, err := ResolvePath(c, "raw", true); err != nil || p != "/_knowledge/raw.md" {
		t.Errorf("knowledge-only raw = %q %v", p, err)
	}
	if p, err := ResolvePath(c, "unit", false); err != nil || p != "/_knowledge/unit.md" {
		t.Errorf("unit = %q %v", p, err)
	}
	_, err = ResolvePath(c, "nothing", false)
	wantErr(t, err, "no note found")
	_, err = ResolvePath(c, "dup", true)
	wantErr(t, err, "no knowledge note named")
	_, err = ResolvePath(c, "/raw.md", true)
	wantErr(t, err, "not a knowledge note")
	if p, err := ResolvePath(c, "/raw.md", false); err != nil || p != "/raw.md" {
		t.Errorf("path = %q %v", p, err)
	}

	path, content, err := Read(c, "/raw.md", false)
	if err != nil || path != "/raw.md" || content != "raw body" {
		t.Errorf("Read = %q %q %v", path, content, err)
	}
	_, err = Resolve(c, "missing")
	wantErr(t, err, "no note found with name")
}

func TestCreate(t *testing.T) {
	f, c := newFakeVault()
	data := func(s string) func() ([]byte, error) { return func() ([]byte, error) { return []byte(s), nil } }
	called := false
	spy := func() ([]byte, error) { called = true; return []byte("x"), nil }

	_, err := Create(c, "/raw.md", false, spy)
	wantErr(t, err, "already exists")
	_, err = Create(c, "/elsewhere/raw.md", false, spy)
	wantErr(t, err, "named \"raw.md\" already exists")
	_, err = Create(c, "/n.txt", false, spy)
	wantErr(t, err, "not a markdown file")
	if called {
		t.Error("content must not be read when path checks fail")
	}

	n, err := Create(c, "/new.md", false, data("l1\nl2\n"))
	if err != nil || n != 2 || f.written["/new.md"] != "l1\nl2\n" {
		t.Errorf("create: n=%d err=%v written=%v", n, err, f.written)
	}
	if _, err := Create(c, "/raw.md", true, data("over")); err != nil || f.written["/raw.md"] != "over" {
		t.Errorf("force create: %v %v", err, f.written)
	}
	_, err = Create(c, "/bin.md", true, func() ([]byte, error) { return []byte{0xff, 0xfe, 0xfd}, nil })
	wantErr(t, err, "binary")
}

func TestSearchKnowledgeFilterAndLimit(t *testing.T) {
	f, c := newFakeVault()
	for i := 0; i < 6; i++ {
		kind := "knowledge"
		if i%2 == 0 {
			kind = ""
		}
		f.hits = append(f.hits, client.SearchResult{Name: string(rune('a' + i)), Kind: kind, Score: 0.123456})
	}

	all, err := Search(c, SearchQuery{Query: "q", Limit: 4})
	if err != nil || all.Total != 4 || all.Results[0].Score != 0.123 {
		t.Errorf("all = %+v %v", all, err)
	}
	k, err := Search(c, SearchQuery{Query: "q", Limit: 2, KnowledgeOnly: true})
	if err != nil || k.Total != 2 {
		t.Fatalf("knowledge = %+v %v", k, err)
	}
	for _, r := range k.Results {
		if r.Kind != "knowledge" {
			t.Errorf("non-knowledge hit leaked: %+v", r)
		}
	}
	_, err = Search(c, SearchQuery{Query: "q", Field: "bogus"})
	wantErr(t, err, "unknown field")

	empty := &fakeVault{notes: nil, written: map[string]string{}}
	res, _ := Search(client.NewInProcess(empty.handler()), SearchQuery{Query: "q"})
	if res.Results == nil {
		t.Error("results must be an empty slice, not nil, so JSON stays []")
	}
}
