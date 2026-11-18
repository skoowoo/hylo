package view

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
)

type stubSearcher struct {
	results []search.SearchResult
}

func (s stubSearcher) Search(_ string, _ search.SearchOptions) ([]search.SearchResult, error) {
	return s.results, nil
}
func (s stubSearcher) TagDocCount(string) (uint64, error)     { return 0, nil }
func (s stubSearcher) TagDistribution(int) ([]search.TagCount, error) {
	return nil, nil
}

func TestShortSearchSnippet(t *testing.T) {
	content := "intro line\nthis is a long enough sentence before redis cache appears #backend\n![[photo.png]]"
	got := shortSearchSnippet(content, []int{2}, "redis")
	if !strings.HasPrefix(got, "…") || !strings.Contains(got, "redis cache") || !strings.Contains(got, "#backend") {
		t.Fatalf("unexpected snippet: %q", got)
	}

	tagOnly := shortSearchSnippet("![[photo.png]]\nhello world #backend", nil, "backend")
	if tagOnly != "hello world #backend" {
		t.Fatalf("tagOnly = %q, want %q", tagOnly, "hello world #backend")
	}

	h1Line := shortSearchSnippet("# Release summary", []int{1}, "Release")
	if h1Line != "Release summary" {
		t.Fatalf("h1Line = %q, want %q", h1Line, "Release summary")
	}

	deepLine := strings.Repeat("word ", 50) + "targettoken after two hundred runes"
	deepGot := shortSearchSnippet(deepLine, []int{1}, `"targettoken"`)
	if !strings.HasPrefix(deepGot, "…") || !strings.Contains(deepGot, "targettoken") {
		t.Fatalf("deepGot = %q, want shifted snippet containing targettoken", deepGot)
	}
}

func TestSearchResultsAndFragmentForShortEntry(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	defer v.Close()

	e1, err := v.AppendShort([]byte("first short about postgres"))
	if err != nil {
		t.Fatalf("AppendShort 1: %v", err)
	}
	e2, err := v.AppendShort([]byte("second short about redis #cache"))
	if err != nil {
		t.Fatalf("AppendShort 2: %v", err)
	}

	p, _ := storage.ParsePath(e2.Path)
	vh := NewView(v, stubSearcher{results: []search.SearchResult{
		{
			Dir:       p.Dir(),
			Name:      p.Base(),
			Kind:      "short",
			ShortID:   e2.ID,
			UpdatedAt: e2.CreatedAt,
			Score:     1.0,
			Lines:     []int{1},
		},
	}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/notes/search?q=redis", nil)
	rec := httptest.NewRecorder()
	vh.SearchResults(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SearchResults status = %d: %s", rec.Code, rec.Body.String())
	}

	var items []searchResultItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode SearchResults: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.ShortID != e2.ID || it.Key != e2.Path+"#"+e2.ID {
		t.Fatalf("item identity = %+v", it)
	}
	if it.Stamp != storage.ShortStamp(e2.CreatedAt) || it.DirLabel != "" {
		t.Fatalf("item meta = %+v", it)
	}
	if !strings.Contains(it.Name, "redis") {
		t.Fatalf("item name = %q, want snippet with redis", it.Name)
	}

	// Fragment preview for that single entry must exclude the sibling entry.
	fragReq := httptest.NewRequest(http.MethodGet, "/notes/fragment?path="+e2.Path+"&short="+e2.ID, nil)
	fragReq.Header.Set("HX-Request", "true")
	fragRec := httptest.NewRecorder()
	vh.NoteFragment(fragRec, fragReq)
	if fragRec.Code != http.StatusOK {
		t.Fatalf("NoteFragment status = %d: %s", fragRec.Code, fragRec.Body.String())
	}
	body := fragRec.Body.String()
	if !strings.Contains(body, "redis") || strings.Contains(body, e1.Content) {
		t.Fatalf("unexpected fragment body: %s", body)
	}
}
