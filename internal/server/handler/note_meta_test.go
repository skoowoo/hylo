package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func postNoteMeta(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(noteMetaRequest{Path: path})
	req := httptest.NewRequest(http.MethodPost, "/api/notes/meta", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestNoteMeta(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/journal/april.md", []byte("# April\n\nbody"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/_knowledge/rag.md", []byte("# RAG"), ""); err != nil {
		t.Fatal(err)
	}
	src, _ := storage.ParsePath("/journal/april.md")
	know, _ := storage.ParsePath("/_knowledge/rag.md")
	if err := v.SetKnowledgeDeps(know, []storage.Path{src}); err != nil {
		t.Fatal(err)
	}

	h := NewNoteMeta(v)

	rec := postNoteMeta(t, h, "/journal/april.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got noteMetaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Dir != "/journal" || got.UpdatedAt == 0 || got.CreatedAt == 0 {
		t.Fatalf("unexpected meta: %+v", got)
	}
	if len(got.Links) != 1 || got.Links[0].Path != "/_knowledge/rag.md" {
		t.Fatalf("links = %+v, want the compiled knowledge note", got.Links)
	}

	rec = postNoteMeta(t, h, "/journal/none.md")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing note status = %d, want 404", rec.Code)
	}
}
