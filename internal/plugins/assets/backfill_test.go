package assets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hardhacker/hylo/internal/storage"
)

func TestBackfillCoversFetchesAndReusesRemote(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()
	p := newTestPlugin(t)
	defer p.Stop()
	raw := []byte("---\nimage_url: " + srv.URL + "/cover.png\n---\n# Remote\n")
	if err := p.vault.WriteNote("/remote.md", raw, ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := p.BackfillCovers(context.Background()); err != nil {
			t.Fatal(err)
		}
		cover, err := p.vault.GetNoteAsset("/remote.md", storage.AssetKindCover)
		if err != nil || cover.Filename != remoteStem(srv.URL+"/cover.png")+".png" {
			t.Fatalf("cover = %+v, err = %v", cover, err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("downloads = %d, want 1", hits.Load())
	}
	if err := p.vault.WriteNote("/remote.md", []byte("# No cover now\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := p.BackfillCovers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.vault.GetNoteAsset("/remote.md", storage.AssetKindCover); !errors.Is(err, storage.ErrMetadataNotFound) {
		t.Fatalf("stale cover: %v", err)
	}
}

func TestBackfillCoversCancellation(t *testing.T) {
	p := newTestPlugin(t)
	defer p.Stop()
	if err := p.vault.WriteNote("/note.md", []byte("# Note\n"), ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.BackfillCovers(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
