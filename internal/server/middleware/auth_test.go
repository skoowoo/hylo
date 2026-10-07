package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func stubLogin(w http.ResponseWriter, _ *http.Request, status int, _, _ string) {
	w.WriteHeader(status)
}

func newAuthServer(t *testing.T) http.Handler {
	t.Helper()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return Authenticator("secret", slog.New(slog.NewTextHandler(io.Discard, nil)), stubLogin)(ok)
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthHeaderAndBearer(t *testing.T) {
	h := newAuthServer(t)

	if got := do(h, httptest.NewRequest("POST", "/api/x", nil)).Code; got != 401 {
		t.Fatalf("no creds: %d", got)
	}
	r := httptest.NewRequest("POST", "/api/x", nil)
	r.Header.Set("X-Hylo-API-Key", "secret")
	if got := do(h, r).Code; got != 200 {
		t.Fatalf("header: %d", got)
	}
	r = httptest.NewRequest("POST", "/api/x", nil)
	r.Header.Set("Authorization", "Bearer secret")
	if got := do(h, r).Code; got != 200 {
		t.Fatalf("bearer: %d", got)
	}
	if got := do(h, httptest.NewRequest("POST", "/healthz", nil)).Code; got != 200 {
		t.Fatalf("healthz: %d", got)
	}
}

func TestBrowserRedirectsToLogin(t *testing.T) {
	h := newAuthServer(t)
	r := httptest.NewRequest("GET", "/home?a=1", nil)
	r.Header.Set("Accept", "text/html")
	rec := do(h, r)
	if rec.Code != 302 || rec.Header().Get("Location") != "/login?next="+url.QueryEscape("/home?a=1") {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	r = httptest.NewRequest("GET", "/home/section", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("Accept", "text/html")
	rec = do(h, r)
	if rec.Code != 401 || rec.Header().Get("HX-Redirect") != "/login" {
		t.Fatalf("htmx: %d %q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
}

func TestLoginSetsUsableCookie(t *testing.T) {
	h := newAuthServer(t)
	form := url.Values{"key": {"secret"}, "next": {"/home"}}
	r := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := do(h, r)
	if rec.Code != 303 || rec.Header().Get("Location") != "/home" {
		t.Fatalf("login: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Value == "secret" {
		t.Fatalf("bad cookie: %+v", cookies)
	}

	r = httptest.NewRequest("POST", "/api/x", nil)
	r.AddCookie(cookies[0])
	if got := do(h, r).Code; got != 200 {
		t.Fatalf("cookie auth: %d", got)
	}
}

func TestLoginRejectsWrongKeyAndOpenRedirect(t *testing.T) {
	h := newAuthServer(t)
	form := url.Values{"key": {"nope"}, "next": {"//evil.com"}}
	r := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := do(h, r).Code; got != 401 {
		t.Fatalf("wrong key: %d", got)
	}
	for _, bad := range []string{"//evil.com", "https://evil.com", "/\t/evil.com", "/\n/evil.com", "/\\evil.com", "evil"} {
		if safeNext(bad) != homePath {
			t.Fatalf("open redirect allowed: %q", bad)
		}
	}
	if safeNext("/home?x=1") != "/home?x=1" {
		t.Fatal("same-origin path rejected")
	}
}

func TestClientIPTrustsForwardedOnlyFromLoopback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "9.9.9.9")
	if got := ClientIP(r); got != "9.9.9.9" {
		t.Fatalf("got %s", got)
	}
	r.RemoteAddr = "8.8.8.8:1234"
	if got := ClientIP(r); got != "8.8.8.8" {
		t.Fatalf("got %s", got)
	}
}

func TestLoopbackIsNotExempt(t *testing.T) {
	h := newAuthServer(t)
	r := httptest.NewRequest("POST", "http://127.0.0.1:54321/api/x", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	if got := do(h, r).Code; got != 401 {
		t.Fatalf("loopback must authenticate too: %d", got)
	}
	r.Header.Set("X-Hylo-API-Key", "secret")
	if got := do(h, r).Code; got != 200 {
		t.Fatalf("loopback with key: %d", got)
	}
}

func TestStaticAssetsSkipAuth(t *testing.T) {
	h := newAuthServer(t)
	if got := do(h, httptest.NewRequest("GET", "/static/vendor/htmx.min.js", nil)).Code; got != 200 {
		t.Fatalf("static: %d", got)
	}
	if got := do(h, httptest.NewRequest("POST", "/static/x", nil)).Code; got != 401 {
		t.Fatalf("non-GET static must authenticate: %d", got)
	}
}
