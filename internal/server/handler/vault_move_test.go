package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestVaultMove(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/inbox/note.md", []byte("hi"), ""); err != nil {
		t.Fatal(err)
	}

	gh := NewVault(v)
	body, _ := json.Marshal(moveRequest{Path: "/inbox/note.md", NewDir: "/archive"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/move", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.Move(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Path != "/archive/note.md" {
		t.Fatalf("path = %q, want /archive/note.md", resp.Path)
	}
}

func TestVaultMoveConflict(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/a/note.md", []byte("a"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/b/note.md", []byte("b"), ""); err != nil {
		t.Fatal(err)
	}

	gh := NewVault(v)
	body, _ := json.Marshal(moveRequest{Path: "/a/note.md", NewDir: "/b"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/move", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.Move(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestVaultMoveMissingFields(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	gh := NewVault(v)

	for _, body := range []moveRequest{
		{Path: "", NewDir: "/x"},
		{Path: "/a.md", NewDir: ""},
	} {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/vault/move", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		gh.Move(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %+v: status = %d, want 400", body, rec.Code)
		}
	}
}
