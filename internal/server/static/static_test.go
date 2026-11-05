package static

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerCaching(t *testing.T) {
	h := http.StripPrefix("/static/", Handler())
	get := func(target, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", target, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	versioned := get(URL("vendor/htmx.min.js"), "")
	if versioned.Code != 200 || !strings.Contains(versioned.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("versioned: %d %q", versioned.Code, versioned.Header().Get("Cache-Control"))
	}
	if ct := versioned.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("content type: %q", ct)
	}

	bare := get("/static/vendor/htmx.min.js", "")
	if bare.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("bare: %q", bare.Header().Get("Cache-Control"))
	}
	if got := get("/static/vendor/htmx.min.js", bare.Header().Get("ETag")).Code; got != http.StatusNotModified {
		t.Fatalf("revalidate: %d", got)
	}

	stale := get("/static/vendor/htmx.min.js?v=0", "")
	if stale.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("stale version must not be immutable: %q", stale.Header().Get("Cache-Control"))
	}

	if got := get("/static/vendor/", "").Code; got != http.StatusNotFound {
		t.Fatalf("directory: %d", got)
	}
}

func TestHandlerGzip(t *testing.T) {
	h := http.StripPrefix("/static/", Handler())
	req := httptest.NewRequest("GET", URL("vendor/htmx.min.js"), nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("gzip: %q %q", rec.Header().Get("Content-Encoding"), rec.Header().Get("Content-Type"))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(zr)
	if !bytes.Equal(got, files["vendor/htmx.min.js"].body) {
		t.Fatal("gzip body differs from the original")
	}

	plain := httptest.NewRequest("GET", URL("vendor/htmx.min.js"), nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, plain)
	if rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("plain: %q vary %q", rec.Header().Get("Content-Encoding"), rec.Header().Get("Vary"))
	}
}
