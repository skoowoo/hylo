package static

import (
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
