package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestVaultCreateUntitled(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	gh := NewVault(v)
	body, _ := json.Marshal(createUntitledRequest{Content: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/create-untitled", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.CreateUntitled(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Path, "/Untitled ") {
		t.Fatalf("path = %q, want an /Untitled ... note", resp.Path)
	}

	content, err := v.ReadNote(storage.Path(resp.Path))
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
}

func TestVaultCreateUntitledRejectsBadDir(t *testing.T) {
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	gh := NewVault(v)
	body, _ := json.Marshal(createUntitledRequest{Dir: "relative/path"})
	req := httptest.NewRequest(http.MethodPost, "/api/vault/create-untitled", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	gh.CreateUntitled(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}
