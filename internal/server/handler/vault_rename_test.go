package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestVaultRename(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/journal/old.md", []byte("hi"), ""); err != nil {
		t.Fatal(err)
	}

	gh := NewVault(v)
	body, _ := json.Marshal(renameRequest{Path: "/journal/old.md", NewName: "new"}) // bare name, no ".md"
	req := httptest.NewRequest(http.MethodPost, "/api/vault/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.Rename(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path        string `json:"path"`
		RenameJobID int64  `json:"renameJobId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Path != "/journal/new.md" {
		t.Fatalf("path = %q, want /journal/new.md", resp.Path)
	}
	if resp.RenameJobID == 0 {
		t.Fatal("expected a non-zero renameJobId")
	}

	// The status endpoint should resolve the id the rename just handed back.
	statusReq := httptest.NewRequest(http.MethodGet, "/api/vault/rename-status?id=1", nil)
	statusRec := httptest.NewRecorder()
	gh.RenameStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status endpoint: code = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}
}

func TestVaultRenameConflictIsVaultWide(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.WriteNote("/a/old.md", []byte("a"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("/b/taken.md", []byte("b"), ""); err != nil {
		t.Fatal(err)
	}

	gh := NewVault(v)
	body, _ := json.Marshal(renameRequest{Path: "/a/old.md", NewName: "taken.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.Rename(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestVaultRenameNotAllowedForSystemNote(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	knowledge := storage.Path("/_knowledge/k.md")
	if err := v.WriteNote(knowledge, []byte("---\nkind: knowledge\n---\n\nbody\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := v.MarkNoteAsKnowledge(knowledge, "", 0, nil); err != nil {
		t.Fatal(err)
	}

	gh := NewVault(v)
	body, _ := json.Marshal(renameRequest{Path: "/_knowledge/k.md", NewName: "renamed.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.Rename(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestVaultRenameMissingFields(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	gh := NewVault(v)

	for _, body := range []renameRequest{
		{Path: "", NewName: "x.md"},
		{Path: "/a.md", NewName: ""},
		{Path: "/a.md", NewName: "sub/b.md"},
	} {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/vault/rename", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		gh.Rename(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %+v: status = %d, want 400, body = %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestVaultRenameStatusUnknownID(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	gh := NewVault(v)

	req := httptest.NewRequest(http.MethodGet, "/api/vault/rename-status?id=999", nil)
	rec := httptest.NewRecorder()
	gh.RenameStatus(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
