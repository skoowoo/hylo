package assets

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/storage"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)

func TestBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "224.0.0.1"}
	for _, s := range blocked {
		if !blockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "198.18.0.1"} {
		if blockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestRejectInternalHostLiteral(t *testing.T) {
	if err := rejectInternalHost("127.0.0.1"); err == nil {
		t.Fatal("loopback literal must be rejected")
	}
	if err := rejectInternalHost("8.8.8.8"); err != nil {
		t.Fatalf("public literal rejected: %v", err)
	}
}

func TestFetchClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach an internal server")
	}))
	defer srv.Close()

	if _, err := newFetchClient().Get(srv.URL); err == nil {
		t.Fatal("expected loopback fetch to be rejected")
	}
}

func TestGuardBlocksRedirectToInternalHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.0.0.1/secret.png", http.StatusFound)
	}))
	defer srv.Close()

	// Allow only the test server's own loopback host on the first hop.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: guardTransport{base: http.DefaultTransport, check: func(host string) error {
			if host == "127.0.0.1" {
				return nil
			}
			return rejectInternalHost(host)
		}},
	}
	if _, err := client.Get(srv.URL); err == nil {
		t.Fatal("expected redirect to internal host to be rejected")
	}
}

// newTestPlugin bypasses the SSRF guard so tests can use httptest servers.
func newTestPlugin(t *testing.T) *Plugin {
	t.Helper()
	vault, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	p := New(vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.client = &http.Client{Timeout: 5 * time.Second}
	return p
}

func TestFetchRemoteSavesOnceAndReuses(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	p := newTestPlugin(t)
	first, err := p.fetchRemote(srv.URL + "/a.png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.fetchRemote(srv.URL + "/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != second.Name || hits.Load() != 1 {
		t.Fatalf("expected reuse: names %q/%q, hits %d", first.Name, second.Name, hits.Load())
	}
	if want := remoteStem(srv.URL+"/a.png") + ".png"; first.Name != want {
		t.Fatalf("name = %q, want %q", first.Name, want)
	}
	other, err := p.fetchRemote(srv.URL + "/b.png")
	if err != nil {
		t.Fatal(err)
	}
	if other.Name == first.Name {
		t.Fatal("different URLs must not share a filename")
	}

	if err := p.vault.DeleteImage(first.Dir, first.Name); err != nil {
		t.Fatal(err)
	}
	third, err := p.fetchRemote(srv.URL + "/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if third.Name != first.Name || hits.Load() != 3 {
		t.Fatalf("deleted image must be re-downloaded under the same name: %q/%q, hits %d", first.Name, third.Name, hits.Load())
	}
}

func TestFetchRemoteRejects(t *testing.T) {
	cases := []struct {
		name, path, contentType, body string
	}{
		{"svg by content type", "/a", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg"/>`},
		{"svg by extension", "/a.svg", "application/octet-stream", `<svg/>`},
		{"json", "/a.json", "application/json", `{"a":1}`},
		{"html labelled png", "/a.png", "image/png", `<html></html>`},
		{"json labelled avif", "/a", "image/avif", `{"secret":true}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", c.contentType)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			if _, err := newTestPlugin(t).fetchRemote(srv.URL + c.path); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestResolveCoverLocalMustBeRegistered(t *testing.T) {
	p := newTestPlugin(t)
	if _, _, ok := p.resolveCover("/n.md", coverSource{local: "missing.png"}); ok {
		t.Fatal("unregistered local image must not become a cover")
	}
	img, err := p.vault.SaveImageBytes(pngBytes, ".png")
	if err != nil {
		t.Fatal(err)
	}
	name, src, ok := p.resolveCover("/n.md", coverSource{local: img.Name})
	if !ok || name != img.Name || src != "" {
		t.Fatalf("got %q %q %v", name, src, ok)
	}
}

func TestExtractCoverFallsBackToNextCandidate(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	p := newTestPlugin(t)
	img, err := p.vault.SaveImageBytes(pngBytes, ".png")
	if err != nil {
		t.Fatal(err)
	}
	raw := "---\nimage_url: \"" + srv.URL + "/dead.png\"\n---\n\n![[" + img.Name + "]]\n"

	sp := storage.Path("/n.md")
	p.extractCover(sp, string(sp), []byte(raw))

	got, err := p.vault.GetNoteAsset(sp, storage.AssetKindCover)
	if err != nil || got.Filename != img.Name {
		t.Fatalf("cover = %+v, err = %v; want %s", got, err, img.Name)
	}
}

func TestExtractCoverUpdatesAndClearsExisting(t *testing.T) {
	p := newTestPlugin(t)
	first, err := p.vault.SaveImageBytes(pngBytes, ".png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.vault.SaveImageBytes(pngBytes, ".png")
	if err != nil {
		t.Fatal(err)
	}

	sp := storage.Path("/n.md")
	p.extractCover(sp, string(sp), []byte("![["+first.Name+"]]\n"))
	got, err := p.vault.GetNoteAsset(sp, storage.AssetKindCover)
	if err != nil || got.Filename != first.Name {
		t.Fatalf("initial cover = %+v, err = %v; want %s", got, err, first.Name)
	}

	p.extractCover(sp, string(sp), []byte("![["+second.Name+"]]\n"))
	got, err = p.vault.GetNoteAsset(sp, storage.AssetKindCover)
	if err != nil || got.Filename != second.Name {
		t.Fatalf("updated cover = %+v, err = %v; want %s", got, err, second.Name)
	}
	covers, err := p.vault.ListNoteAssets(sp, storage.AssetKindCover)
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) != 1 {
		t.Fatalf("cover rows = %d, want 1: %+v", len(covers), covers)
	}

	p.extractCover(sp, string(sp), []byte("no cover here\n"))
	_, err = p.vault.GetNoteAsset(sp, storage.AssetKindCover)
	if !errors.Is(err, storage.ErrMetadataNotFound) {
		t.Fatalf("cleared cover err = %v, want ErrMetadataNotFound", err)
	}
}
