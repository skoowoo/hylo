package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestartRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[server]\nproxy = \"not a proxy\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var n int
	h := NewRestartHTTP(path, func() { n++ })
	rec := httptest.NewRecorder()
	h.Post(rec, httptest.NewRequest(http.MethodPost, "/api/server/restart", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if n != 0 {
		t.Fatalf("trigger called %d times", n)
	}
}

func TestRestartAcceptsValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[server]\nhost = \"127.0.0.1\"\nport = 54321\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var n int
	h := NewRestartHTTP(path, func() { n++ })
	rec := httptest.NewRecorder()
	h.Post(rec, httptest.NewRequest(http.MethodPost, "/api/server/restart", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if n != 1 {
		t.Fatalf("trigger called %d times", n)
	}
	if !strings.Contains(rec.Body.String(), `"started_at":"`+processStarted+`"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}
