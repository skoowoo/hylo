package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hardhacker/vaultr/internal/storage"
)

func TestNoteInfo(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/journal/april.md", []byte("# April\n\nsome body text"), ""); err != nil {
		t.Fatal(err)
	}

	h := NewNoteInfo(v)
	body, _ := json.Marshal(noteInfoRequest{Paths: []string{"/journal/april.md", "/missing.md"}})
	req := httptest.NewRequest(http.MethodPost, "/api/notes/info", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Notes []noteInfoRow `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Notes) != 1 {
		t.Fatalf("notes = %+v, want exactly the one existing path", resp.Notes)
	}
	got := resp.Notes[0]
	if got.Path != "/journal/april.md" || got.Title != "april.md" || got.Dir != "/journal" {
		t.Fatalf("unexpected row: %+v", got)
	}
}
