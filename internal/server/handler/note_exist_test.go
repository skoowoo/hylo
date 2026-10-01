package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestNoteExist(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/alive.md", []byte("body"), ""); err != nil {
		t.Fatal(err)
	}

	h := NewNoteExist(v)

	body, _ := json.Marshal(existRequest{Names: []string{"alive.md", "gone.md"}})
	req := httptest.NewRequest(http.MethodPost, "/api/notes/exist", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Existing []string `json:"existing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Existing) != 1 || resp.Existing[0] != "alive.md" {
		t.Fatalf("existing = %v, want [alive.md]", resp.Existing)
	}
}

func TestNoteExistEmptyNames(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	h := NewNoteExist(v)
	body, _ := json.Marshal(existRequest{Names: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/notes/exist", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Existing []string `json:"existing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Existing) != 0 {
		t.Fatalf("existing = %v, want empty", resp.Existing)
	}
}
