package urlembed

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/storage"
)

func TestCanonical(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"https://Example.com/a#x", "https://example.com/a", true},
		{"https://example.com", "https://example.com/", true},
		{"http://example.com:80/a", "http://example.com/a", true},
		{"https://example.com:443/a", "https://example.com/a", true},
		{"https://example.com:8443/a", "https://example.com:8443/a", true},
		{"https://example.com/a?b=1&c=2", "https://example.com/a?b=1&c=2", true},
		{"http://[::1]/", "http://[::1]/", true},
		{"ftp://example.com/a", "", false},
		{"https://user:pass@example.com/a", "", false},
		{"not a url", "", false},
		{"/relative", "", false},
	}
	for _, tc := range cases {
		got, err := canonical(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Errorf("canonical(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("canonical(%q) = %q, want error", tc.in, got)
		}
	}
}

func TestPublicIP(t *testing.T) {
	// 198.18/15 is where Clash fake-ip DNS points every name.
	ok := []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "64:ff9b::808:808", "2002:808:808::1", "198.18.0.5"}
	bad := []string{
		"127.0.0.1", "10.1.2.3", "192.168.1.1", "172.16.0.1", "169.254.169.254", "::1", "0.0.0.0", "255.255.255.255",
		"100.64.0.1", "::ffff:127.0.0.1", "240.0.0.1", "192.0.2.1",
		"64:ff9b::a9fe:a9fe", "64:ff9b::7f00:1", "2002:a00:1::1", "2002:7f00:1::", "64:ff9b:1::1",
	}
	for _, s := range ok {
		if !publicIP(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
	for _, s := range bad {
		if publicIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
}

func TestRedirectPolicy(t *testing.T) {
	req := &http.Request{URL: mustURL("http://169.254.169.254/latest")}
	if err := redirectPolicy(req, nil); err == nil {
		t.Fatal("link-local redirect allowed")
	}
	via := make([]*http.Request, 5)
	req = &http.Request{URL: mustURL("https://example.com/next")}
	if err := redirectPolicy(req, via); err == nil {
		t.Fatal("6th request allowed")
	}
}

func TestResolveClassifiesAndCaches(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/story":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head>
				<meta property="og:title" content="A &amp; B">
				<meta property="og:description" content="The lead">
				<meta property="og:site_name" content="Example">
				<meta property="og:image" content="/og.jpg">
				<title>ignored</title>
			</head><body>x</body></html>`))
		case "/go":
			http.Redirect(w, r, "/story", http.StatusFound)
		case "/ep.mp3":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("not-audio"))
		case "/clip.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
		case "/pic.png":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
		case "/doc.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.WriteHeader(http.StatusOK)
		case "/page.mp3":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>Player</title></head></html>`))
		case "/down":
			http.Error(w, "nope", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	v := newVault(t)
	r := NewResolver(v, config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	clock := time.Now()
	r.now = func() time.Time { return clock }
	ctx := context.Background()

	page := r.Resolve(ctx, srv.URL+"/go")
	if page.Status != "ok" || page.Kind != kindPage || page.Title != "A & B" || page.Description != "The lead" || page.SiteName != "Example" {
		t.Fatalf("page = %+v", page)
	}
	if page.ImageURL != srv.URL+"/og.jpg" {
		t.Fatalf("image = %q", page.ImageURL)
	}
	if page.FinalURL != srv.URL+"/story" {
		t.Fatalf("final = %q", page.FinalURL)
	}
	if hits.Load() != 2 {
		t.Fatalf("redirect hits = %d, want 2", hits.Load())
	}
	again := r.Resolve(ctx, srv.URL+"/go")
	if again.Title != "A & B" || hits.Load() != 2 {
		t.Fatalf("cache miss: hits %d title %q", hits.Load(), again.Title)
	}

	if got := r.Resolve(ctx, srv.URL+"/ep.mp3"); got.Kind != kindAudio {
		t.Fatalf("audio = %+v", got)
	}
	if got := r.Resolve(ctx, srv.URL+"/clip.mp4"); got.Kind != kindVideo {
		t.Fatalf("video = %+v", got)
	}
	if got := r.Resolve(ctx, srv.URL+"/pic.png"); got.Kind != kindImage {
		t.Fatalf("image = %+v", got)
	}
	if got := r.Resolve(ctx, srv.URL+"/doc.pdf"); got.Kind != kindFile || got.ContentType != "application/pdf" {
		t.Fatalf("file = %+v", got)
	}
	if got := r.Resolve(ctx, srv.URL+"/page.mp3"); got.Kind != kindPage || got.Title != "Player" {
		t.Fatalf("html at a media path = %+v", got)
	}

	down := r.Resolve(ctx, srv.URL+"/down")
	if down.Status != "error" || down.Error != "fetch_failed" {
		t.Fatalf("down = %+v", down)
	}
	before := hits.Load()
	if again := r.Resolve(ctx, srv.URL+"/down"); again.Error != "fetch_failed" || hits.Load() != before {
		t.Fatalf("error not cached: hits %d -> %d", before, hits.Load())
	}
	var retried Result
	r.ResolveEach(ctx, []string{srv.URL + "/down"}, true, func(res Result) { retried = res })
	if retried.Error != "fetch_failed" || hits.Load() != before+1 {
		t.Fatalf("retry did not refetch: hits %d -> %d", before, hits.Load())
	}
	before = hits.Load()
	clock = clock.Add(2 * time.Hour)
	if again := r.Resolve(ctx, srv.URL+"/down"); hits.Load() == before {
		t.Fatalf("expired error not refetched: %+v", again)
	}

	before = hits.Load()
	clock = clock.Add(8 * 24 * time.Hour)
	back := r.Resolve(ctx, srv.URL+"/go")
	if back.Title != "A & B" || hits.Load() < before+2 {
		t.Fatalf("expired page not refetched: %+v hits %d -> %d", back, before, hits.Load())
	}
}

func TestResolveSingleflight(t *testing.T) {
	var hits atomic.Int32
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-hold
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Once</title></head></html>`))
	}))
	defer srv.Close()

	r := NewResolver(newVault(t), config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	ctx := context.Background()
	done := make(chan Result, 2)
	go func() { done <- r.Resolve(ctx, srv.URL+"/slow") }()
	waitHits(t, &hits, 1)
	go func() { done <- r.Resolve(ctx, srv.URL+"/slow") }()
	time.Sleep(50 * time.Millisecond)
	if hits.Load() != 1 {
		t.Fatalf("second request started, hits=%d", hits.Load())
	}
	close(hold)
	a, b := <-done, <-done
	if a.Title != "Once" || b.Title != "Once" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

func TestResolveDecodesCharset(t *testing.T) {
	title, err := simplifiedchinese.GBK.NewEncoder().String("今日新闻")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><meta charset="gbk"><title>` + title + `</title></head></html>`))
	}))
	defer srv.Close()
	r := NewResolver(newVault(t), config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	if got := r.Resolve(context.Background(), srv.URL+"/"); got.Title != "今日新闻" {
		t.Fatalf("title = %q", got.Title)
	}
}

// The card needs only the head; the rest of a large page is never waited on.
func TestResolveStopsAtHead(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		pad := strings.Repeat("x", 5000)
		_, _ = w.Write([]byte(`<html><head><title>Fast</title><meta name="description" content="` + pad + `"></head><body>`))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer srv.Close()
	defer close(release)
	r := NewResolver(newVault(t), config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	done := make(chan Result, 1)
	go func() { done <- r.Resolve(context.Background(), srv.URL+"/") }()
	select {
	case got := <-done:
		if got.Title != "Fast" {
			t.Fatalf("got %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("resolve waited for the body")
	}
}

func TestResolveKeepsGoodRowThroughOutage(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "x", status)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Kept</title></head></html>`))
	}))
	defer srv.Close()
	r := NewResolver(newVault(t), config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	clock := time.Now()
	r.now = func() time.Time { return clock }
	ctx := context.Background()
	if got := r.Resolve(ctx, srv.URL+"/"); got.Title != "Kept" {
		t.Fatalf("first = %+v", got)
	}

	status = http.StatusServiceUnavailable
	clock = clock.Add(8 * 24 * time.Hour)
	if got := r.Resolve(ctx, srv.URL+"/"); got.Status != "ok" || got.Title != "Kept" {
		t.Fatalf("outage = %+v", got)
	}
	rows, err := r.vault.FreshURLEmbeds([]string{srv.URL + "/"}, clock.UnixMilli())
	if err != nil || rows[srv.URL+"/"].ExpiresAt != clock.Add(errorTTL).UnixMilli() {
		t.Fatalf("row = %+v, %v", rows, err)
	}

	status = http.StatusNotFound
	clock = clock.Add(2 * time.Hour)
	if got := r.Resolve(ctx, srv.URL+"/"); got.Status != "error" {
		t.Fatalf("gone = %+v", got)
	}
}

func TestResolveEachEmitsCachedFirst(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Slow</title></head></html>`))
	}))
	defer srv.Close()
	r := NewResolver(newVault(t), config.Proxy{})
	r.allowPrivate = true
	r.client = srv.Client()
	now := time.Now().UnixMilli()
	if err := r.vault.PutURLEmbed(storage.URLEmbed{
		URL: "https://example.com/", Kind: kindPage, Title: "Cached", Status: "ok",
		FetchedAt: now, ExpiresAt: now + int64(time.Hour/time.Millisecond),
	}); err != nil {
		t.Fatal(err)
	}
	got := make(chan Result, 2)
	go r.ResolveEach(context.Background(), []string{srv.URL + "/slow", "https://example.com/"}, false, func(res Result) { got <- res })
	select {
	case first := <-got:
		if first.Title != "Cached" {
			t.Fatalf("first = %+v", first)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cached row waited for the slow fetch")
	}
	close(release)
	if second := <-got; second.Title != "Slow" {
		t.Fatalf("second = %+v", second)
	}
}

// The proxy itself is on loopback; reaching it must not count as a private dial.
func TestResolveThroughProxy(t *testing.T) {
	var asked atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Store(r.URL.String())
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Via proxy</title></head></html>`))
	}))
	defer proxy.Close()
	parsed, err := config.ParseProxy(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(newVault(t), parsed)
	got := r.Resolve(context.Background(), "http://1.1.1.1/page")
	if got.Title != "Via proxy" || asked.Load() != "http://1.1.1.1/page" {
		t.Fatalf("got %+v, proxy asked %v", got, asked.Load())
	}
	if got := r.Resolve(context.Background(), "http://10.0.0.1/"); got.Error != "blocked" {
		t.Fatalf("private through proxy = %+v", got)
	}
}

func TestResolveBlocksPrivateURLs(t *testing.T) {
	v := newVault(t)
	r := NewResolver(v, config.Proxy{})
	for _, raw := range []string{"http://127.0.0.1/", "http://localhost/x", "http://10.1.2.3/a", "http://169.254.169.254/"} {
		got := r.Resolve(context.Background(), raw)
		if got.Status != "error" || got.Error != "blocked" {
			t.Fatalf("%s = %+v", raw, got)
		}
	}
	rows, err := v.FreshURLEmbeds([]string{"http://127.0.0.1/"}, time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if rows["http://127.0.0.1/"].Error != "blocked" {
		t.Fatalf("blocked row not cached: %+v", rows)
	}
}

func waitHits(t *testing.T, hits *atomic.Int32, n int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() < n {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the request")
		}
		time.Sleep(time.Millisecond)
	}
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func newVault(t *testing.T) *storage.Vault {
	t.Helper()
	v, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}
